package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConfigManagerForValidation(t *testing.T) *ConfigManager {
	t.Helper()
	dir := t.TempDir()
	historyDir := filepath.Join(dir, "history")
	if err := os.MkdirAll(historyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return &ConfigManager{
		configDir:       dir,
		syncModeFile:    filepath.Join(dir, "sync_mode.json"),
		appSettingsFile: filepath.Join(dir, "app_settings.json"),
		quickCmdFile:    filepath.Join(dir, "quick_commands.json"),
		paramHistFile:   filepath.Join(dir, "param_history.json"),
		historyDir:      historyDir,
		globalHistFile:  filepath.Join(historyDir, "global.json"),
	}
}

func TestSyncModeValidationAndNormalization(t *testing.T) {
	cm := testConfigManagerForValidation(t)
	if err := cm.SetSyncMode(" SFTP "); err != nil {
		t.Fatalf("SetSyncMode valid normalized value: %v", err)
	}
	if got := cm.GetSyncMode(); got != "sftp" {
		t.Fatalf("GetSyncMode() = %q, want sftp", got)
	}
	if err := cm.SetSyncMode("unknown"); err == nil {
		t.Fatal("SetSyncMode accepted an unsupported provider")
	}
	if got := cm.GetSyncMode(); got != "sftp" {
		t.Fatalf("invalid update changed persisted mode to %q", got)
	}
	if err := os.WriteFile(cm.syncModeFile, []byte(`"corrupt-provider"`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := cm.GetSyncMode(); got != "webdav" {
		t.Fatalf("invalid persisted mode = %q, want safe default webdav", got)
	}
}

func TestMigrateAITasksDirRejectsNestedTarget(t *testing.T) {
	cm := testConfigManagerForValidation(t)
	source := filepath.Join(cm.configDir, "tasks")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "conversation.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(source, "migrated")
	err := cm.MigrateAITasksDir(target)
	if err == nil || !strings.Contains(err.Error(), "内部") {
		t.Fatalf("nested migration target error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "conversation.json")); err != nil {
		t.Fatalf("source data changed after rejected migration: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("rejected target should not be created, stat error = %v", err)
	}
}

func TestConfigJSONWritersRejectInvalidPayloads(t *testing.T) {
	cm := testConfigManagerForValidation(t)
	tests := []struct {
		name string
		call func() error
	}{
		{"quick commands", func() error { return cm.SaveQuickCommandsLocal(`{"broken"`) }},
		{"parameter history", func() error { return cm.SaveParamHistory(`not-json`) }},
		{"command history", func() error { return cm.SaveCommandHistory("terminal-1", `[`) }},
		{"global command history", func() error { return cm.SaveGlobalCommandHistory(`]`) }},
		{"empty command history session", func() error { return cm.SaveCommandHistory("", `[]`) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); err == nil {
				t.Fatal("invalid payload was accepted")
			}
		})
	}
}

func TestSaveConnectionWithErrorPropagatesWriteFailure(t *testing.T) {
	cm := testConfigManagerForValidation(t)
	cm.connFile = filepath.Join(cm.configDir, "missing", "connections.json")
	_, err := cm.SaveConnectionWithError(Connection{Host: "example.com", Port: 22, Username: "alice"}, false)
	if err == nil {
		t.Fatal("SaveConnectionWithError reported success when its directory was unavailable")
	}
}
