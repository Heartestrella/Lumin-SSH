package wailsapp

import (
	"testing"

	"lumeterm/internal/config"
)

func TestShutdownRunsOnce(t *testing.T) {
	calls := 0
	app := &App{
		onBeforeQuit: func() {
			calls++
		},
	}

	app.shutdown()
	app.shutdown()

	if calls != 1 {
		t.Fatalf("退出清理回调执行次数 = %d，期望 1", calls)
	}
}

func TestValidateConnectionForSave(t *testing.T) {
	valid := config.Connection{Host: "example.com", Port: 22, Username: "alice"}
	if err := validateConnectionForSave(valid); err != nil {
		t.Fatalf("valid connection rejected: %v", err)
	}
	for name, conn := range map[string]config.Connection{
		"missing host":     {Port: 22, Username: "alice"},
		"zero port":        {Host: "example.com", Username: "alice"},
		"oversized port":   {Host: "example.com", Port: 65536, Username: "alice"},
		"missing identity": {Host: "example.com", Port: 22},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateConnectionForSave(conn); err == nil {
				t.Fatal("invalid connection was accepted")
			}
		})
	}
	if err := validateConnectionForSave(config.Connection{Host: "example.com", Port: 22, CredentialID: "cred-1"}); err != nil {
		t.Fatalf("credential-backed connection rejected: %v", err)
	}
}

func TestValidateSerialConfig(t *testing.T) {
	if err := validateSerialConfig("session-1", "COM3", 115200, 8, 1, "none"); err != nil {
		t.Fatalf("valid serial config rejected: %v", err)
	}
	invalid := []struct {
		name      string
		sessionID string
		portName  string
		baudRate  int
		dataBits  int
		stopBits  float64
		parity    string
	}{
		{"missing session", "", "COM3", 9600, 8, 1, "none"},
		{"missing port", "s", "", 9600, 8, 1, "none"},
		{"bad baud", "s", "COM3", 0, 8, 1, "none"},
		{"bad data bits", "s", "COM3", 9600, 9, 1, "none"},
		{"bad stop bits", "s", "COM3", 9600, 8, 3, "none"},
		{"bad parity", "s", "COM3", 9600, 8, 1, "invalid"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSerialConfig(tc.sessionID, tc.portName, tc.baudRate, tc.dataBits, tc.stopBits, tc.parity); err == nil {
				t.Fatal("invalid serial config was accepted")
			}
		})
	}
}
