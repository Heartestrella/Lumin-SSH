package mcp

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type overlapDetectWriter struct {
	active     atomic.Int32
	overlapped atomic.Bool
}

type blockingWriteCloser struct {
	started   chan struct{}
	closed    chan struct{}
	startOnce sync.Once
	closeOnce sync.Once
}

func (w *blockingWriteCloser) Write(p []byte) (int, error) {
	w.startOnce.Do(func() { close(w.started) })
	<-w.closed
	return 0, io.ErrClosedPipe
}

func (w *blockingWriteCloser) Close() error {
	w.closeOnce.Do(func() { close(w.closed) })
	return nil
}

func (w *overlapDetectWriter) Write(p []byte) (int, error) {
	if w.active.Add(1) > 1 {
		w.overlapped.Store(true)
	}
	time.Sleep(time.Millisecond)
	w.active.Add(-1)
	return len(p), nil
}

func (w *overlapDetectWriter) Close() error {
	return nil
}

func TestStdioTransportSerializesConcurrentWrites(t *testing.T) {
	transport := newStdioTransport(ServerConfig{}, nil)
	writer := &overlapDetectWriter{}
	transport.stdin = writer
	transport.started.Store(true)

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := transport.Request(context.Background(), "notifications/test", map[string]any{
				"value": strings.Repeat("x", 4096),
			}, nil); err != nil {
				t.Errorf("notification request failed: %v", err)
			}
		}()
	}
	wg.Wait()
	if writer.overlapped.Load() {
		t.Fatal("concurrent requests wrote to stdio at the same time")
	}
}

func TestStdioTransportDoesNotStartAfterClose(t *testing.T) {
	transport := newStdioTransport(ServerConfig{}, nil)
	if err := transport.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if err := transport.Start(context.Background()); err == nil {
		t.Fatal("closed transport must not start a process")
	}
}

func TestStdioTransportCloseInterruptsBlockedWrite(t *testing.T) {
	transport := newStdioTransport(ServerConfig{}, nil)
	writer := &blockingWriteCloser{
		started: make(chan struct{}),
		closed:  make(chan struct{}),
	}
	transport.stdin = writer
	transport.started.Store(true)

	requestDone := make(chan error, 1)
	go func() {
		requestDone <- transport.Request(context.Background(), "notifications/test", nil, nil)
	}()
	<-writer.started

	closeDone := make(chan error, 1)
	go func() {
		closeDone <- transport.Close()
	}()
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("close failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close waited for a blocked stdio write")
	}
	select {
	case err := <-requestDone:
		if err == nil {
			t.Fatal("interrupted write should fail")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked stdio write was not interrupted")
	}
}

func TestStaleConnectionCannotOverwriteReplacementState(t *testing.T) {
	const key = "global::demo"
	hub := &ClientHub{connections: map[string]*serverConnection{}}
	stale := &serverConnection{key: key}
	current := &serverConnection{
		key: key,
		runtime: ServerRuntime{
			Status:       "connected",
			ErrorHistory: []ServerErrorEntry{},
		},
	}
	hub.connections[key] = current

	hub.appendServerLog(stale, "late log", "error")
	hub.markConnectionError(stale, errors.New("late failure"))
	if current.runtime.Status != "connected" {
		t.Fatalf("stale connection changed replacement status to %q", current.runtime.Status)
	}
	if current.runtime.Error != "" || len(current.runtime.ErrorHistory) != 0 {
		t.Fatalf("stale connection polluted replacement state: %#v", current.runtime)
	}

	hub.markConnectionError(current, errors.New("current failure"))
	if current.runtime.Status != "disconnected" || len(current.runtime.ErrorHistory) != 1 {
		t.Fatalf("current connection error was not recorded: %#v", current.runtime)
	}
}
