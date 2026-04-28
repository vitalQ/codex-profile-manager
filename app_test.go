package main

import (
	"os"
	"path/filepath"
	"testing"

	"codex-profile-manager/internal/audit"
	"codex-profile-manager/internal/config"
	"codex-profile-manager/internal/detector"
	"codex-profile-manager/internal/paths"
	"codex-profile-manager/internal/profile"
	"codex-profile-manager/internal/switcher"
)

func TestImportDuplicatePreservesCurrentlyActiveProfile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(targetAuthPath, []byte(`{"token":"same"}`), 0o600); err != nil {
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
	active, err := profileService.CreateFromBytes(profile.CreateInput{Name: "active"}, []byte(`{"token":"same"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes(active) error = %v", err)
	}

	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, audit.NewService(appPaths.AuditLogFile), detectorService)

	app := &App{
		appPaths: appPaths,
		config:   configService,
		profiles: profileService,
		audit:    audit.NewService(appPaths.AuditLogFile),
		detector: detectorService,
		switcher: switcherService,
	}

	duplicate, err := app.ImportProfileFromRaw(ImportProfileFromRawInput{
		Name:    "active copy",
		RawJSON: `{"token":"same"}`,
	})
	if err != nil {
		t.Fatalf("ImportProfileFromRaw() error = %v", err)
	}
	if duplicate.ID == active.ID {
		t.Fatalf("expected duplicate profile to have a different id")
	}

	settings, err := configService.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if settings.ActiveProfileID != active.ID {
		t.Fatalf("settings.ActiveProfileID = %s, want %s", settings.ActiveProfileID, active.ID)
	}

	current, err := detectorService.Current(targetAuthPath, settings.ActiveProfileID)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.ProfileID != active.ID {
		t.Fatalf("current.ProfileID = %s, want %s", current.ProfileID, active.ID)
	}
}

func TestBootstrapUsesProfileSummariesWhileGetProfileReturnsRawJSON(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex", "auth.json")
	if err := os.MkdirAll(filepath.Dir(targetAuthPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(targetAuthPath, []byte(`{"token":"summary"}`), 0o600); err != nil {
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
	record, err := profileService.CreateFromBytes(profile.CreateInput{Name: "summary-test"}, []byte(`{"token":"summary"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	app := &App{
		appPaths: appPaths,
		config:   configService,
		profiles: profileService,
		audit:    audit.NewService(appPaths.AuditLogFile),
		detector: detector.NewService(profileService),
		switcher: switcher.NewService(configService, profileService, audit.NewService(appPaths.AuditLogFile), detector.NewService(profileService)),
	}

	bootstrap, err := app.Bootstrap()
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if len(bootstrap.Profiles) != 1 {
		t.Fatalf("expected 1 profile in bootstrap, got %d", len(bootstrap.Profiles))
	}
	if bootstrap.Profiles[0].RawJSON != "" {
		t.Fatalf("bootstrap profile unexpectedly exposed rawJson")
	}

	detail, err := app.GetProfile(record.ID)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}
	if detail.RawJSON == "" {
		t.Fatalf("expected GetProfile() to return rawJson")
	}
}

func TestPinCurrentActiveProfileAuditsDetectorFailureWithoutBlockingImport(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	targetAuthPath := filepath.Join(tempDir, ".codex")
	if err := os.MkdirAll(targetAuthPath, 0o700); err != nil {
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
	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	app := &App{
		appPaths: appPaths,
		config:   configService,
		profiles: profileService,
		audit:    auditService,
		detector: detectorService,
		switcher: switcherService,
	}

	record, err := app.ImportProfileFromRaw(ImportProfileFromRawInput{
		Name:    "import-still-works",
		RawJSON: `{"token":"new"}`,
	})
	if err != nil {
		t.Fatalf("ImportProfileFromRaw() error = %v", err)
	}
	if record.ID == "" {
		t.Fatalf("expected imported profile id")
	}

	entries, err := auditService.List(10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected at least one audit entry")
	}
	entry := entries[0]
	if entry.Action != "pin_active_profile" {
		t.Fatalf("entry.Action = %s, want pin_active_profile", entry.Action)
	}
	if entry.Result != "warning" {
		t.Fatalf("entry.Result = %s, want warning", entry.Result)
	}
	if entry.TargetPath != targetAuthPath {
		t.Fatalf("entry.TargetPath = %s, want %s", entry.TargetPath, targetAuthPath)
	}
}
