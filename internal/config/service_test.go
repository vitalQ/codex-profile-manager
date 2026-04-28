package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"codex-profile-manager/internal/config"
	"codex-profile-manager/internal/paths"
)

func TestSaveAndLoadSettings(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	service, err := config.NewService(paths.AppPaths{
		BaseDir:      tempDir,
		SettingsFile: filepath.Join(tempDir, "settings.json"),
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	saved, err := service.Save(config.Settings{
		TargetAuthPath:           filepath.Join(tempDir, ".codex", "auth.json"),
		ActiveProfileID:          "profile-123",
		LaunchCodexAfterSwitch:   true,
		AllowPlaintextExport:     true,
		EnableSessionHistorySync: false,
		Theme:                    "dark",
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := service.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded != saved {
		t.Fatalf("loaded settings mismatch: %#v vs %#v", loaded, saved)
	}
}

func TestLoadSettingsDefaultsHistorySyncWhenMissing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{
  "targetAuthPath": "C:\\temp\\.codex\\auth.json",
  "launchCodexAfterSwitch": false,
  "allowPlaintextExport": false,
  "theme": "system"
}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	service, err := config.NewService(paths.AppPaths{
		BaseDir:      tempDir,
		SettingsFile: settingsPath,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	loaded, err := service.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !loaded.EnableSessionHistorySync {
		t.Fatalf("expected EnableSessionHistorySync to default to true when field is missing")
	}
}
