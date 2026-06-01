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
	if strings.Contains(content, `model_provider = "OpenAI"`) {
		t.Fatalf("expected model_provider to be left unmanaged: %s", content)
	}
	if !strings.Contains(content, `[model_providers.OpenAI]`) {
		t.Fatalf("expected OpenAI provider section in managed block: %s", content)
	}
	if !strings.Contains(content, `base_url = "https://example.com/v1"`) {
		t.Fatalf("expected base_url in managed block: %s", content)
	}
	if count := strings.Count(content, `supports_websockets = true`); count != 1 {
		t.Fatalf("expected one supports_websockets line, got %d: %s", count, content)
	}

	state, err := codexcfg.ReadManagedCustomProvider(path)
	if err != nil {
		t.Fatalf("ReadManagedCustomProvider() error = %v", err)
	}
	if !state.Present || state.BaseURL != "https://example.com/v1" {
		t.Fatalf("unexpected managed state: %#v", state)
	}
	if state.Provider != "OpenAI" {
		t.Fatalf("state.Provider = %q, want OpenAI", state.Provider)
	}
}

func TestEnsureManagedCustomProviderReplacesExistingBlock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`model_provider = "OpenAI"`,
		"",
		`foo = "bar"`,
		"",
		codexcfg.StartMarker,
		`[model_providers.OpenAI]`,
		`name = "OpenAI"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`supports_websockets = true`,
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
	if !strings.Contains(content, strings.Join([]string{
		`model_provider = "OpenAI"`,
		"",
		`foo = "bar"`,
	}, "\n")) {
		t.Fatalf("expected existing model_provider position to be preserved: %s", content)
	}
	if !strings.Contains(content, `[model_providers.OpenAI]`) {
		t.Fatalf("expected OpenAI provider section: %s", content)
	}
	if !strings.Contains(content, `base_url = "https://new.example/v1"`) {
		t.Fatalf("expected updated base_url: %s", content)
	}
	if count := strings.Count(content, `supports_websockets = true`); count != 1 {
		t.Fatalf("expected one supports_websockets line, got %d: %s", count, content)
	}
	if !strings.Contains(content, `foo = "bar"`) {
		t.Fatalf("expected other config content to be preserved")
	}
}

func TestEnsureManagedCustomProviderKeepsExistingOpenAIModelProviderLine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`notify = ["node", "notify-hook.js"]`,
		`model_reasoning_effort = "high"`,
		"",
		`model_provider = "OpenAI"`,
		codexcfg.StartMarker,
		`[model_providers.OpenAI]`,
		`name = "OpenAI"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`supports_websockets = true`,
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
	if !strings.Contains(content, `notify = ["node", "notify-hook.js"]`) {
		t.Fatalf("expected top-level config to be preserved: %s", content)
	}
	if !strings.Contains(content, strings.Join([]string{
		`model_reasoning_effort = "high"`,
		"",
		`model_provider = "OpenAI"`,
		codexcfg.StartMarker,
	}, "\n")) {
		t.Fatalf("expected model_provider line to keep its original position: %s", content)
	}
	if !strings.Contains(content, `base_url = "https://new.example/v1"`) {
		t.Fatalf("expected updated base_url: %s", content)
	}
}

func TestEnsureManagedCustomProviderUpdatesExistingModelProviderLine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`notify = ["node", "notify-hook.js"]`,
		`model_provider = "openai"`,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := codexcfg.EnsureManagedCustomProvider(path, "https://example.com/v1"); err != nil {
		t.Fatalf("EnsureManagedCustomProvider() error = %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(payload)
	if !strings.Contains(content, strings.Join([]string{
		`notify = ["node", "notify-hook.js"]`,
		`model_provider = "OpenAI"`,
		"",
	}, "\n")) {
		t.Fatalf("expected existing model_provider line to be updated in place: %s", content)
	}
	if strings.Contains(content, `model_provider = "openai"`) {
		t.Fatalf("expected old model_provider value to be replaced: %s", content)
	}
	if !strings.Contains(content, `[model_providers.OpenAI]`) {
		t.Fatalf("expected OpenAI provider section: %s", content)
	}
}

func TestEnsureManagedCustomProviderDoesNotUpdateNestedModelProviderLine(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`[custom]`,
		`model_provider = "openai"`,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := codexcfg.EnsureManagedCustomProvider(path, "https://example.com/v1"); err != nil {
		t.Fatalf("EnsureManagedCustomProvider() error = %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(payload)
	if !strings.Contains(content, strings.Join([]string{
		`[custom]`,
		`model_provider = "openai"`,
	}, "\n")) {
		t.Fatalf("expected nested model_provider line to be preserved: %s", content)
	}
	if strings.Contains(content, `model_provider = "OpenAI"`) {
		t.Fatalf("expected no top-level model_provider to be inserted: %s", content)
	}
}

func TestRemoveManagedCustomProviderKeepsOtherConfigAndRestoresModelProvider(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	content := strings.Join([]string{
		`model_provider = "OpenAI"`,
		"",
		`foo = "bar"`,
		"",
		codexcfg.StartMarker,
		`[model_providers.OpenAI]`,
		`name = "OpenAI"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`supports_websockets = true`,
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
	if !strings.Contains(updated, `model_provider = "openai"`) {
		t.Fatalf("expected model_provider line to be restored to official provider: %s", updated)
	}
	if strings.Contains(updated, `model_provider = "OpenAI"`) {
		t.Fatalf("expected custom model_provider value to be removed: %s", updated)
	}
	if !strings.Contains(updated, `foo = "bar"`) || !strings.Contains(updated, `bar = "baz"`) {
		t.Fatalf("expected surrounding config to remain: %s", updated)
	}
}

func TestRemoveManagedCustomProviderDoesNotInsertModelProvider(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	content := strings.Join([]string{
		`foo = "bar"`,
		"",
		codexcfg.StartMarker,
		`[model_providers.OpenAI]`,
		`name = "OpenAI"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`supports_websockets = true`,
		`base_url = "https://example.com/v1"`,
		codexcfg.EndMarker,
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
	if strings.Contains(updated, `model_provider`) {
		t.Fatalf("expected model_provider line not to be inserted: %s", updated)
	}
	if !strings.Contains(updated, `foo = "bar"`) {
		t.Fatalf("expected surrounding config to remain: %s", updated)
	}
}

func TestEnsureManagedCustomProviderRejectsUnmanagedConflict(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`model_provider = "OpenAI"`,
		"",
		`[model_providers.OpenAI]`,
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

func TestEnsureManagedCustomProviderRejectsLegacyManagedProvider(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`model_provider = "custom"`,
		"",
		codexcfg.StartMarker,
		`[model_providers.custom]`,
		`name = "custom"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`base_url = "https://legacy.example/v1"`,
		codexcfg.EndMarker,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := codexcfg.EnsureManagedCustomProvider(path, "https://example.com/v1")
	if err == nil {
		t.Fatalf("expected legacy managed provider error")
	}
}

func TestEnsureManagedCustomProviderPreservesExtraKeys(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.toml")
	// Existing config with user-added extra keys inside managed block.
	if err := os.WriteFile(path, []byte(strings.Join([]string{
		`model_provider = "OpenAI"`,
		"",
		codexcfg.StartMarker,
		`[model_providers.OpenAI]`,
		`name = "OpenAI"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`base_url = "https://old.example/v1"`,
		`supports_websockets = true`,
		`supports_streaming = true`,
		codexcfg.EndMarker,
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Switch to a new base URL — extra keys should be preserved.
	if err := codexcfg.EnsureManagedCustomProvider(path, "https://new.example/v1"); err != nil {
		t.Fatalf("EnsureManagedCustomProvider() error = %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(payload)
	if !strings.Contains(content, `base_url = "https://new.example/v1"`) {
		t.Fatalf("expected updated base_url: %s", content)
	}
	if count := strings.Count(content, `supports_websockets = true`); count != 1 {
		t.Fatalf("expected one supports_websockets line, got %d: %s", count, content)
	}
	if !strings.Contains(content, `supports_streaming = true`) {
		t.Fatalf("expected supports_streaming to be preserved: %s", content)
	}
	if strings.Contains(content, "old.example") {
		t.Fatalf("expected old base_url to be replaced: %s", content)
	}
	if !strings.Contains(content, `model_provider = "OpenAI"`) || !strings.Contains(content, `[model_providers.OpenAI]`) {
		t.Fatalf("expected OpenAI provider to remain managed: %s", content)
	}
}

func TestParseExtraLines(t *testing.T) {
	t.Parallel()

	block := strings.Join([]string{
		codexcfg.StartMarker,
		`[model_providers.OpenAI]`,
		`name = "OpenAI"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`supports_websockets = true`,
		`base_url = "https://example.com/v1"`,
		`custom_header = "X-My-Header"`,
		codexcfg.EndMarker,
	}, "\n")

	extra := codexcfg.ParseExtraLines(block)

	if len(extra) != 1 {
		t.Fatalf("expected 1 extra line, got %d: %v", len(extra), extra)
	}
	if extra[0] != `custom_header = "X-My-Header"` {
		t.Fatalf("expected custom_header line, got: %s", extra[0])
	}
}
