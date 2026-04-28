package codexcfg_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codex-profile-manager/internal/codexcfg"
)

func TestEnsureManagedCustomProviderAppendsAndReadsBlock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := codexcfg.EnsureManagedCustomProvider(path, "https://example.com/v1"); err != nil {
		t.Fatalf("EnsureManagedCustomProvider() error = %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(payload)
	if !strings.Contains(content, codexcfg.StartMarker) {
		t.Fatalf("expected managed start marker")
	}
	if !strings.HasPrefix(content, "model_provider = \"custom\"\n") {
		t.Fatalf("expected model_provider to be inserted at first line: %s", content)
	}
	if !strings.Contains(content, `base_url = "https://example.com/v1"`) {
		t.Fatalf("expected base_url in managed block: %s", content)
	}

	state, err := codexcfg.ReadManagedCustomProvider(path)
	if err != nil {
		t.Fatalf("ReadManagedCustomProvider() error = %v", err)
	}
	if !state.Present || state.BaseURL != "https://example.com/v1" {
		t.Fatalf("unexpected managed state: %#v", state)
	}
}

func TestEnsureManagedCustomProviderReplacesExistingBlock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`model_provider = "custom"`,
		"",
		`foo = "bar"`,
		"",
		codexcfg.StartMarker,
		`[model_providers.custom]`,
		`name = "custom"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`base_url = "https://old.example/v1"`,
		codexcfg.EndMarker,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := codexcfg.EnsureManagedCustomProvider(path, "https://new.example/v1"); err != nil {
		t.Fatalf("EnsureManagedCustomProvider() error = %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(payload)
	if strings.Contains(content, "old.example") {
		t.Fatalf("expected old base_url to be replaced: %s", content)
	}
	if !strings.HasPrefix(content, "model_provider = \"custom\"\n") {
		t.Fatalf("expected model_provider at first line: %s", content)
	}
	if !strings.Contains(content, `base_url = "https://new.example/v1"`) {
		t.Fatalf("expected updated base_url: %s", content)
	}
	if !strings.Contains(content, `foo = "bar"`) {
		t.Fatalf("expected other config content to be preserved")
	}
}

func TestRemoveManagedCustomProviderKeepsOtherConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	content := strings.Join([]string{
		`model_provider = "custom"`,
		"",
		`foo = "bar"`,
		"",
		codexcfg.StartMarker,
		`[model_providers.custom]`,
		`name = "custom"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`base_url = "https://example.com/v1"`,
		codexcfg.EndMarker,
		"",
		`bar = "baz"`,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := codexcfg.RemoveManagedCustomProvider(path); err != nil {
		t.Fatalf("RemoveManagedCustomProvider() error = %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	updated := string(payload)
	if strings.Contains(updated, codexcfg.StartMarker) || strings.Contains(updated, codexcfg.EndMarker) {
		t.Fatalf("expected managed block to be removed: %s", updated)
	}
	if strings.Contains(updated, `model_provider = "custom"`) {
		t.Fatalf("expected managed model_provider line to be removed: %s", updated)
	}
	if !strings.Contains(updated, `foo = "bar"`) || !strings.Contains(updated, `bar = "baz"`) {
		t.Fatalf("expected surrounding config to remain: %s", updated)
	}
}

func TestEnsureManagedCustomProviderRejectsUnmanagedConflict(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`model_provider = "custom"`,
		"",
		`[model_providers.custom]`,
		`base_url = "https://manual.example/v1"`,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := codexcfg.EnsureManagedCustomProvider(path, "https://example.com/v1")
	if err == nil {
		t.Fatalf("expected unmanaged conflict error")
	}
}
