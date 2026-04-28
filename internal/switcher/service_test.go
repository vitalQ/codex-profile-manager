package switcher_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codex-profile-manager/internal/audit"
	"codex-profile-manager/internal/codexcfg"
	"codex-profile-manager/internal/codexsession"
	"codex-profile-manager/internal/config"
	"codex-profile-manager/internal/detector"
	"codex-profile-manager/internal/paths"
	"codex-profile-manager/internal/profile"
	"codex-profile-manager/internal/switcher"
)

func TestSwitchProfileReplacesTargetAndMarksManagedState(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(targetAuthPath, []byte(`{"token":"old"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	appPaths := paths.AppPaths{
		BaseDir:           tempDir,
		ProfilesDir:       filepath.Join(tempDir, "profiles"),
		LogsDir:           filepath.Join(tempDir, "logs"),
		SettingsFile:      filepath.Join(tempDir, "settings.json"),
		ProfilesIndexFile: filepath.Join(tempDir, "profiles.json"),
		AuditLogFile:      filepath.Join(tempDir, "logs", "audit.jsonl"),
	}
	for _, dir := range []string{appPaths.BaseDir, appPaths.ProfilesDir, appPaths.LogsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	configService, err := config.NewService(appPaths)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := configService.Save(config.Settings{
		TargetAuthPath:         targetAuthPath,
		LaunchCodexAfterSwitch: false,
		AllowPlaintextExport:   false,
		Theme:                  "system",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profileService := profile.NewService(appPaths.ProfilesIndexFile)
	record, err := profileService.CreateFromBytes(profile.CreateInput{Name: "new-profile"}, []byte(`{"token":"new"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	result, err := switcherService.SwitchProfile(record.ID)
	if err != nil {
		t.Fatalf("SwitchProfile() error = %v", err)
	}

	payload, err := os.ReadFile(targetAuthPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(payload) != `{"token":"new"}` {
		t.Fatalf("unexpected target payload: %s", string(payload))
	}

	if !result.Current.Managed || result.Current.ProfileID != record.ID {
		t.Fatalf("unexpected current state: %#v", result.Current)
	}

	settings, err := configService.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if settings.ActiveProfileID != record.ID {
		t.Fatalf("settings.ActiveProfileID = %s, want %s", settings.ActiveProfileID, record.ID)
	}
}

func TestSwitchProfileAPIKeyWritesManagedConfig(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	appPaths := paths.AppPaths{
		BaseDir:           tempDir,
		ProfilesDir:       filepath.Join(tempDir, "profiles"),
		LogsDir:           filepath.Join(tempDir, "logs"),
		SettingsFile:      filepath.Join(tempDir, "settings.json"),
		ProfilesIndexFile: filepath.Join(tempDir, "profiles.json"),
		AuditLogFile:      filepath.Join(tempDir, "logs", "audit.jsonl"),
	}
	for _, dir := range []string{appPaths.BaseDir, appPaths.ProfilesDir, appPaths.LogsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	configService, err := config.NewService(appPaths)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := configService.Save(config.Settings{
		TargetAuthPath:           targetAuthPath,
		EnableSessionHistorySync: true,
		Theme:                    "system",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profileService := profile.NewService(appPaths.ProfilesIndexFile)
	record, err := profileService.CreateFromBytes(profile.CreateInput{
		Name:    "api-provider",
		Mode:    profile.ModeAPIKey,
		BaseURL: "https://example.com/v1",
	}, []byte(`{"OPENAI_API_KEY":"sk-123"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	if _, err := switcherService.SwitchProfile(record.ID); err != nil {
		t.Fatalf("SwitchProfile() error = %v", err)
	}

	configPayload, err := os.ReadFile(codexcfg.ConfigPathForAuthPath(targetAuthPath))
	if err != nil {
		t.Fatalf("ReadFile(config.toml) error = %v", err)
	}
	if !strings.Contains(string(configPayload), `base_url = "https://example.com/v1"`) {
		t.Fatalf("expected config.toml to contain managed base_url: %s", string(configPayload))
	}
}

func TestSwitchProfileOfficialRemovesManagedConfig(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	configPath := codexcfg.ConfigPathForAuthPath(targetAuthPath)
	if err := codexcfg.EnsureManagedCustomProvider(configPath, "https://example.com/v1"); err != nil {
		t.Fatalf("EnsureManagedCustomProvider() error = %v", err)
	}

	appPaths := paths.AppPaths{
		BaseDir:           tempDir,
		ProfilesDir:       filepath.Join(tempDir, "profiles"),
		LogsDir:           filepath.Join(tempDir, "logs"),
		SettingsFile:      filepath.Join(tempDir, "settings.json"),
		ProfilesIndexFile: filepath.Join(tempDir, "profiles.json"),
		AuditLogFile:      filepath.Join(tempDir, "logs", "audit.jsonl"),
	}
	for _, dir := range []string{appPaths.BaseDir, appPaths.ProfilesDir, appPaths.LogsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	configService, err := config.NewService(appPaths)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := configService.Save(config.Settings{
		TargetAuthPath:           targetAuthPath,
		EnableSessionHistorySync: true,
		Theme:                    "system",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profileService := profile.NewService(appPaths.ProfilesIndexFile)
	record, err := profileService.CreateFromBytes(profile.CreateInput{Name: "official"}, []byte(`{"token":"new"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	if _, err := switcherService.SwitchProfile(record.ID); err != nil {
		t.Fatalf("SwitchProfile() error = %v", err)
	}

	state, err := codexcfg.ReadManagedCustomProvider(configPath)
	if err != nil {
		t.Fatalf("ReadManagedCustomProvider() error = %v", err)
	}
	if state.Present {
		t.Fatalf("expected managed custom provider config to be removed")
	}
}

func TestSwitchProfileKeepsSelectedProfileActiveWhenDuplicatesExist(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	appPaths := paths.AppPaths{
		BaseDir:           tempDir,
		ProfilesDir:       filepath.Join(tempDir, "profiles"),
		LogsDir:           filepath.Join(tempDir, "logs"),
		SettingsFile:      filepath.Join(tempDir, "settings.json"),
		ProfilesIndexFile: filepath.Join(tempDir, "profiles.json"),
		AuditLogFile:      filepath.Join(tempDir, "logs", "audit.jsonl"),
	}
	for _, dir := range []string{appPaths.BaseDir, appPaths.ProfilesDir, appPaths.LogsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	configService, err := config.NewService(appPaths)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := configService.Save(config.Settings{
		TargetAuthPath:         targetAuthPath,
		LaunchCodexAfterSwitch: false,
		AllowPlaintextExport:   false,
		Theme:                  "system",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profileService := profile.NewService(appPaths.ProfilesIndexFile)
	first, err := profileService.CreateFromBytes(profile.CreateInput{Name: "first"}, []byte(`{"token":"same"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes(first) error = %v", err)
	}
	second, err := profileService.CreateFromBytes(profile.CreateInput{Name: "second"}, []byte(`{"token":"same"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes(second) error = %v", err)
	}

	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	result, err := switcherService.SwitchProfile(first.ID)
	if err != nil {
		t.Fatalf("SwitchProfile() error = %v", err)
	}
	if result.Current.ProfileID != first.ID {
		t.Fatalf("result.Current.ProfileID = %s, want %s", result.Current.ProfileID, first.ID)
	}

	current, err := detectorService.Current(targetAuthPath, first.ID)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.ProfileID != first.ID {
		t.Fatalf("current.ProfileID = %s, want %s (duplicate was %s)", current.ProfileID, first.ID, second.ID)
	}
}

func TestSwitchProfileClonesSessionsAcrossProviderTypes(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeSessionFixture(t, filepath.Dir(targetAuthPath), "2026-04-12T21-59-50", "11111111-1111-1111-1111-111111111111", codexsession.ProviderOpenAI)

	appPaths := paths.AppPaths{
		BaseDir:           tempDir,
		ProfilesDir:       filepath.Join(tempDir, "profiles"),
		LogsDir:           filepath.Join(tempDir, "logs"),
		SettingsFile:      filepath.Join(tempDir, "settings.json"),
		ProfilesIndexFile: filepath.Join(tempDir, "profiles.json"),
		AuditLogFile:      filepath.Join(tempDir, "logs", "audit.jsonl"),
	}
	for _, dir := range []string{appPaths.BaseDir, appPaths.ProfilesDir, appPaths.LogsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	configService, err := config.NewService(appPaths)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := configService.Save(config.Settings{
		TargetAuthPath:           targetAuthPath,
		EnableSessionHistorySync: true,
		Theme:                    "system",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profileService := profile.NewService(appPaths.ProfilesIndexFile)
	record, err := profileService.CreateFromBytes(profile.CreateInput{
		Name:    "api-provider",
		Mode:    profile.ModeAPIKey,
		BaseURL: "https://example.com/v1",
	}, []byte(`{"OPENAI_API_KEY":"sk-123"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	result, err := switcherService.SwitchProfile(record.ID)
	if err != nil {
		t.Fatalf("SwitchProfile() error = %v", err)
	}
	if !result.SessionSync.Ran {
		t.Fatalf("expected session sync to run")
	}
	if result.SessionSync.Cloned != 1 {
		t.Fatalf("result.SessionSync.Cloned = %d, want 1", result.SessionSync.Cloned)
	}

	clones := findRolloutFiles(t, filepath.Join(filepath.Dir(targetAuthPath), "sessions"))
	if len(clones) != 2 {
		t.Fatalf("expected 2 rollout files after switch, got %d", len(clones))
	}

	clonePayload := readNonOriginalClonePayload(t, clones, "11111111-1111-1111-1111-111111111111")
	if got := clonePayload["model_provider"]; got != codexsession.ProviderCustom {
		t.Fatalf("clone model_provider = %v, want %s", got, codexsession.ProviderCustom)
	}
}

func TestSwitchProfileSameProviderDoesNotCloneSessions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeSessionFixture(t, filepath.Dir(targetAuthPath), "2026-04-12T21-59-50", "11111111-1111-1111-1111-111111111111", codexsession.ProviderOpenAI)

	appPaths := paths.AppPaths{
		BaseDir:           tempDir,
		ProfilesDir:       filepath.Join(tempDir, "profiles"),
		LogsDir:           filepath.Join(tempDir, "logs"),
		SettingsFile:      filepath.Join(tempDir, "settings.json"),
		ProfilesIndexFile: filepath.Join(tempDir, "profiles.json"),
		AuditLogFile:      filepath.Join(tempDir, "logs", "audit.jsonl"),
	}
	for _, dir := range []string{appPaths.BaseDir, appPaths.ProfilesDir, appPaths.LogsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	configService, err := config.NewService(appPaths)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := configService.Save(config.Settings{
		TargetAuthPath: targetAuthPath,
		Theme:          "system",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profileService := profile.NewService(appPaths.ProfilesIndexFile)
	record, err := profileService.CreateFromBytes(profile.CreateInput{Name: "official"}, []byte(`{"token":"new"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	result, err := switcherService.SwitchProfile(record.ID)
	if err != nil {
		t.Fatalf("SwitchProfile() error = %v", err)
	}
	if result.SessionSync.Ran {
		t.Fatalf("expected session sync to be skipped")
	}
	if len(findRolloutFiles(t, filepath.Join(filepath.Dir(targetAuthPath), "sessions"))) != 1 {
		t.Fatalf("expected session files to remain unchanged")
	}
}

func TestSwitchProfileDoesNotCloneSessionsWhenHistorySyncDisabled(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeSessionFixture(t, filepath.Dir(targetAuthPath), "2026-04-12T21-59-50", "11111111-1111-1111-1111-111111111111", codexsession.ProviderOpenAI)

	appPaths := paths.AppPaths{
		BaseDir:           tempDir,
		ProfilesDir:       filepath.Join(tempDir, "profiles"),
		LogsDir:           filepath.Join(tempDir, "logs"),
		SettingsFile:      filepath.Join(tempDir, "settings.json"),
		ProfilesIndexFile: filepath.Join(tempDir, "profiles.json"),
		AuditLogFile:      filepath.Join(tempDir, "logs", "audit.jsonl"),
	}
	for _, dir := range []string{appPaths.BaseDir, appPaths.ProfilesDir, appPaths.LogsDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	configService, err := config.NewService(appPaths)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := configService.Save(config.Settings{
		TargetAuthPath:           targetAuthPath,
		EnableSessionHistorySync: false,
		Theme:                    "system",
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	profileService := profile.NewService(appPaths.ProfilesIndexFile)
	record, err := profileService.CreateFromBytes(profile.CreateInput{
		Name:    "api-provider",
		Mode:    profile.ModeAPIKey,
		BaseURL: "https://example.com/v1",
	}, []byte(`{"OPENAI_API_KEY":"sk-123"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	result, err := switcherService.SwitchProfile(record.ID)
	if err != nil {
		t.Fatalf("SwitchProfile() error = %v", err)
	}
	if result.SessionSync.Ran {
		t.Fatalf("expected session sync to be disabled")
	}
	if len(findRolloutFiles(t, filepath.Join(filepath.Dir(targetAuthPath), "sessions"))) != 1 {
		t.Fatalf("expected session files to remain unchanged")
	}
}

func writeSessionFixture(t *testing.T, codexDir, ts, sessionID, provider string) {
	t.Helper()

	dir := filepath.Join(codexDir, "sessions", "2026", "04", "12")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	path := filepath.Join(dir, "rollout-"+ts+"-"+sessionID+".jsonl")
	content := `{"timestamp":"2026-04-12T13:59:50Z","type":"session_meta","payload":{"id":"` + sessionID + `","timestamp":"2026-04-12T13:59:50Z","cwd":"D:\\projects\\codex-profile-manager","originator":"codex-tui","cli_version":"0.120.0","source":"cli","model_provider":"` + provider + `"}}` + "\n" +
		`{"timestamp":"2026-04-12T14:00:00Z","type":"response_item","payload":{"role":"user","content":[{"text":"hello"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func findRolloutFiles(t *testing.T, sessionsDir string) []string {
	t.Helper()

	files := []string{}
	err := filepath.WalkDir(sessionsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}
	return files
}

func readNonOriginalClonePayload(t *testing.T, paths []string, originalID string) map[string]any {
	t.Helper()

	for _, path := range paths {
		payload := readSessionMetaFromFile(t, path)
		if payload["id"] != originalID {
			return payload
		}
	}
	t.Fatalf("clone payload not found")
	return nil
}

func readSessionMetaFromFile(t *testing.T, path string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	line := raw
	for index, value := range raw {
		if value == '\n' {
			line = raw[:index]
			break
		}
	}

	var obj map[string]any
	if err := json.Unmarshal(line, &obj); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	payload, _ := obj["payload"].(map[string]any)
	return payload
}
