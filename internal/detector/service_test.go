package detector_test

import (
	"os"
	"path/filepath"
	"testing"

	"codex-profile-manager/internal/codexcfg"
	"codex-profile-manager/internal/detector"
	"codex-profile-manager/internal/profile"
)

func TestCurrentMatchesAPIKeyProfileWhenManagedConfigMatchesBaseURL(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(targetAuthPath, []byte(`{"OPENAI_API_KEY":"sk-demo"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(auth.json) error = %v", err)
	}
	if err := codexcfg.EnsureManagedCustomProvider(codexcfg.ConfigPathForAuthPath(targetAuthPath), "https://example.com/v1"); err != nil {
		t.Fatalf("EnsureManagedCustomProvider() error = %v", err)
	}

	profiles := profile.NewService(filepath.Join(tempDir, "profiles.json"))
	record, err := profiles.CreateFromBytes(profile.CreateInput{
		Name:    "api",
		Mode:    profile.ModeAPIKey,
		BaseURL: "https://example.com/v1",
	}, []byte(`{"OPENAI_API_KEY":"sk-demo"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	service := detector.NewService(profiles)
	current, err := service.Current(targetAuthPath, record.ID)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if !current.Managed || current.ProfileID != record.ID {
		t.Fatalf("unexpected current state: %#v", current)
	}
}

func TestCurrentDoesNotMatchOfficialProfileWhenUnmanagedCustomConfigExists(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	configPath := codexcfg.ConfigPathForAuthPath(targetAuthPath)
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(targetAuthPath, []byte(`{"token":"official"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(auth.json) error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte("model_provider = \"custom\"\n\n[model_providers.custom]\nbase_url = \"https://manual.example/v1\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config.toml) error = %v", err)
	}

	profiles := profile.NewService(filepath.Join(tempDir, "profiles.json"))
	record, err := profiles.CreateFromBytes(profile.CreateInput{Name: "official"}, []byte(`{"token":"official"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	service := detector.NewService(profiles)
	current, err := service.Current(targetAuthPath, record.ID)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.Managed {
		t.Fatalf("expected unmanaged current state, got %#v", current)
	}
}
