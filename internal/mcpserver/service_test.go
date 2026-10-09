package mcpserver

import (
	"errors"
	"sync"
	"testing"
)

func TestServiceFollowLatestTerminalConcurrentAccess(t *testing.T) {
	service := NewService(fakeSessionProvider{descriptors: []SessionDescriptor{{SessionID: "session"}}})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(enabled bool) {
			defer wg.Done()
			service.SetFollowLatestTerminal(enabled)
		}(i%2 == 0)
		go func() {
			defer wg.Done()
			_, _ = service.GetConnectedSession("session")
		}()
	}
	wg.Wait()
}

type fakeSessionProvider struct {
	descriptors []SessionDescriptor
	// staleConnKeys 模拟宿主维护的「已关闭终端 id → 所属连接」映射。
	staleConnKeys map[string]string
}

func (p fakeSessionProvider) ListConnectedSessions() ([]SessionDescriptor, error) {
	return p.descriptors, nil
}

// StaleSessionConnKey implements mcpserver.StaleSessionGroupResolver.
func (p fakeSessionProvider) StaleSessionConnKey(sessionID string) (string, bool) {
	connKey, ok := p.staleConnKeys[sessionID]
	return connKey, ok
}

// newMultiTerminalDescriptors 模拟同一服务器(connKey)上的三个终端:
// root 为最初的主终端,child1/child2 为之后新开的标签,latest 应为 child2。
func newMultiTerminalDescriptors() []SessionDescriptor {
	return []SessionDescriptor{
		{SessionID: "session_root", ConnectionRef: "user@host:22", GroupSessionID: "", IsLatestTerminal: false},
		{SessionID: "term_child1", ConnectionRef: "user@host:22", GroupSessionID: "session_root", IsLatestTerminal: false},
		{SessionID: "term_child2", ConnectionRef: "user@host:22", GroupSessionID: "session_root", IsLatestTerminal: true},
		{SessionID: "session_other", ConnectionRef: "other@host:22", GroupSessionID: "", IsLatestTerminal: true},
	}
}

func TestGetConnectedSessionFollowsLatestTerminal(t *testing.T) {
	service := NewService(fakeSessionProvider{descriptors: newMultiTerminalDescriptors()})
	service.SetFollowLatestTerminal(true)

	cases := []struct {
		name      string
		requested string
		expected  string
	}{
		{"旧主终端重定向到最新标签", "session_root", "term_child2"},
		{"较早的子终端重定向到最新标签", "term_child1", "term_child2"},
		{"最新终端保持不变", "term_child2", "term_child2"},
		{"其他服务器的唯一终端不受影响", "session_other", "session_other"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session, err := service.GetConnectedSession(testCase.requested)
			if err != nil {
				t.Fatalf("GetConnectedSession(%q) returned error: %v", testCase.requested, err)
			}
			if session.SessionID != testCase.expected {
				t.Fatalf("GetConnectedSession(%q) = %q, want %q", testCase.requested, session.SessionID, testCase.expected)
			}
		})
	}
}

func TestGetConnectedSessionStaleParentFallsBackToGroupLatest(t *testing.T) {
	service := NewService(fakeSessionProvider{descriptors: []SessionDescriptor{
		// 主终端标签已关闭,但其子终端仍存活:旧 id 应解析到该组最新终端。
		{SessionID: "term_child", ConnectionRef: "user@host:22", GroupSessionID: "session_root", IsLatestTerminal: true},
	}})
	service.SetFollowLatestTerminal(true)

	session, err := service.GetConnectedSession("session_root")
	if err != nil {
		t.Fatalf("GetConnectedSession(stale root) returned error: %v", err)
	}
	if session.SessionID != "term_child" {
		t.Fatalf("stale root resolved to %q, want %q", session.SessionID, "term_child")
	}
}

func TestGetConnectedSessionClosedChildFallsBackViaStaleMapping(t *testing.T) {
	// 旧子终端 term_child1 已关闭,同组仅剩 term_child2(其 GroupSessionID 指向
	// 父会话而非 term_child1):需经「关闭终端 → 连接」映射兜底解析。
	service := NewService(fakeSessionProvider{
		descriptors: []SessionDescriptor{
			{SessionID: "session_root", ConnectionRef: "user@host:22", GroupSessionID: "", IsLatestTerminal: false},
			{SessionID: "term_child2", ConnectionRef: "user@host:22", GroupSessionID: "session_root", IsLatestTerminal: true},
		},
		staleConnKeys: map[string]string{"term_child1": "user@host:22"},
	})
	service.SetFollowLatestTerminal(true)

	session, err := service.GetConnectedSession("term_child1")
	if err != nil {
		t.Fatalf("GetConnectedSession(closed child) returned error: %v", err)
	}
	if session.SessionID != "term_child2" {
		t.Fatalf("closed child resolved to %q, want %q", session.SessionID, "term_child2")
	}
}

func TestGetConnectedSessionUnknownIDStaysUnresolved(t *testing.T) {
	// 未被记录过关闭现场的未知 id 不得路由到任何连接分组。
	service := NewService(fakeSessionProvider{
		descriptors: []SessionDescriptor{
			{SessionID: "session_root", ConnectionRef: "user@host:22", GroupSessionID: "", IsLatestTerminal: true},
		},
		staleConnKeys: map[string]string{},
	})
	service.SetFollowLatestTerminal(true)

	if _, err := service.GetConnectedSession("term_never_existed"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("unknown id should return ErrSessionNotFound, got %v", err)
	}
}

func TestGetConnectedSessionWithoutFollowKeepsExactMatch(t *testing.T) {
	service := NewService(fakeSessionProvider{descriptors: newMultiTerminalDescriptors()})

	session, err := service.GetConnectedSession("session_root")
	if err != nil {
		t.Fatalf("GetConnectedSession returned error: %v", err)
	}
	if session.SessionID != "session_root" {
		t.Fatalf("follow disabled: resolved to %q, want exact match %q", session.SessionID, "session_root")
	}

	if _, err := service.GetConnectedSession("missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("unknown id should return ErrSessionNotFound, got %v", err)
	}
}

func TestListConnectedSessionsExposesLatestFlag(t *testing.T) {
	service := NewService(fakeSessionProvider{descriptors: newMultiTerminalDescriptors()})

	sessions, err := service.ListConnectedSessions()
	if err != nil {
		t.Fatalf("ListConnectedSessions returned error: %v", err)
	}
	latestByGroup := map[string]int{}
	for _, session := range sessions {
		if session.IsLatestTerminal {
			latestByGroup[session.ConnectionRef]++
		}
	}
	for group, count := range latestByGroup {
		if count != 1 {
			t.Fatalf("group %q has %d latest-terminal flags, want exactly 1", group, count)
		}
	}
	if len(latestByGroup) != 2 {
		t.Fatalf("expected 2 groups with a latest terminal, got %d", len(latestByGroup))
	}
}
