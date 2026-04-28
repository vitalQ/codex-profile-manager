package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"codex-profile-manager/internal/audit"
	"codex-profile-manager/internal/config"
	"codex-profile-manager/internal/detector"
	"codex-profile-manager/internal/paths"
	"codex-profile-manager/internal/profile"
	"codex-profile-manager/internal/switcher"
)

// App struct
type App struct {
	ctx      context.Context
	initErr  error
	appPaths paths.AppPaths
	config   *config.Service
	profiles *profile.Service
	audit    *audit.Service
	detector *detector.Service
	switcher *switcher.Service
	tray     *TrayController
}

// NewApp creates a new App application struct
func NewApp() *App {
	result := &App{}

	appPaths, err := paths.Resolve()
	if err != nil {
		result.initErr = err
		return result
	}

	configService, err := config.NewService(appPaths)
	if err != nil {
		result.initErr = err
		return result
	}

	profileService := profile.NewService(appPaths.ProfilesIndexFile)
	auditService := audit.NewService(appPaths.AuditLogFile)
	detectorService := detector.NewService(profileService)
	switcherService := switcher.NewService(configService, profileService, auditService, detectorService)

	result.appPaths = appPaths
	result.config = configService
	result.profiles = profileService
	result.audit = auditService
	result.detector = detectorService
	result.switcher = switcherService
	if !trayDisabledByEnv() {
		result.tray = NewTrayController(result)
	}

	return result
}

// startup is called when the app starts. The context is saved
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	if a.tray != nil {
		a.tray.Shutdown()
	}
}

func (a *App) ensureReady() error {
	if a.initErr != nil {
		return a.initErr
	}
	if a.config == nil {
		return fmt.Errorf("应用尚未初始化")
	}
	return nil
}

func (a *App) Bootstrap() (BootstrapData, error) {
	if err := a.ensureReady(); err != nil {
		return BootstrapData{}, err
	}
	settings, err := a.config.Load()
	if err != nil {
		return BootstrapData{}, err
	}
	profilesList, err := a.profiles.List()
	if err != nil {
		return BootstrapData{}, err
	}
	current, err := a.detector.Current(settings.TargetAuthPath, settings.ActiveProfileID)
	if err != nil {
		return BootstrapData{}, err
	}

	return BootstrapData{
		Settings: settings,
		Profiles: mapProfiles(profilesList),
		Current:  mapCurrent(current),
	}, nil
}

func (a *App) GetSettings() (config.Settings, error) {
	if err := a.ensureReady(); err != nil {
		return config.Settings{}, err
	}
	return a.config.Load()
}

func (a *App) SaveSettings(input config.Settings) (config.Settings, error) {
	if err := a.ensureReady(); err != nil {
		return config.Settings{}, err
	}
	settings, err := a.config.Save(input)
	if err != nil {
		return config.Settings{}, err
	}
	a.notifyStateChanged("settings-saved")
	return settings, nil
}

func (a *App) PickAuthPath() (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}

	defaultDir := filepath.Dir(a.appPaths.SettingsFile)
	settings, err := a.config.Load()
	if err == nil && strings.TrimSpace(settings.TargetAuthPath) != "" {
		defaultDir = filepath.Dir(settings.TargetAuthPath)
	}

	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "选择目标 auth.json 文件",
		DefaultDirectory: defaultDir,
		DefaultFilename:  "auth.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
}

func (a *App) PickImportFile() (string, error) {
	if err := a.ensureReady(); err != nil {
		return "", err
	}

	homeDir, _ := os.UserHomeDir()
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "选择要导入的 auth.json",
		DefaultDirectory: homeDir,
		DefaultFilename:  "auth.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
}

func (a *App) ListProfiles() ([]ProfileDTO, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	records, err := a.profiles.List()
	if err != nil {
		return nil, err
	}
	return mapProfiles(records), nil
}

func (a *App) GetProfile(id string) (ProfileDTO, error) {
	if err := a.ensureReady(); err != nil {
		return ProfileDTO{}, err
	}
	record, err := a.profiles.Get(id)
	if err != nil {
		return ProfileDTO{}, err
	}
	return mapProfileDetail(record), nil
}

func (a *App) ImportProfileFromCurrentAuth(input ImportProfileInput) (ProfileDTO, error) {
	if err := a.ensureReady(); err != nil {
		return ProfileDTO{}, err
	}
	if err := a.pinCurrentActiveProfile(); err != nil {
		return ProfileDTO{}, err
	}

	settings, err := a.config.Load()
	if err != nil {
		return ProfileDTO{}, err
	}

	payload, err := os.ReadFile(settings.TargetAuthPath)
	if err != nil {
		return ProfileDTO{}, fmt.Errorf("读取当前 auth.json 失败: %w", err)
	}

	record, err := a.profiles.CreateFromBytes(profile.CreateInput{
		Name:     input.Name,
		Mode:     input.Mode,
		Homepage: input.Homepage,
		BaseURL:  input.BaseURL,
		Tags:     input.Tags,
		Note:     input.Note,
	}, payload)
	if err != nil {
		return ProfileDTO{}, err
	}
	a.notifyStateChanged("profile-imported")
	return mapProfileDetail(record), nil
}

func (a *App) ImportProfileFromFile(input ImportProfileFromFileInput) (ProfileDTO, error) {
	if err := a.ensureReady(); err != nil {
		return ProfileDTO{}, err
	}
	if err := a.pinCurrentActiveProfile(); err != nil {
		return ProfileDTO{}, err
	}

	payload, err := os.ReadFile(strings.TrimSpace(input.FilePath))
	if err != nil {
		return ProfileDTO{}, fmt.Errorf("读取导入文件失败: %w", err)
	}

	record, err := a.profiles.CreateFromBytes(profile.CreateInput{
		Name:     input.Name,
		Mode:     input.Mode,
		Homepage: input.Homepage,
		BaseURL:  input.BaseURL,
		Tags:     input.Tags,
		Note:     input.Note,
	}, payload)
	if err != nil {
		return ProfileDTO{}, err
	}
	a.notifyStateChanged("profile-imported")
	return mapProfileDetail(record), nil
}

func (a *App) ImportProfileFromRaw(input ImportProfileFromRawInput) (ProfileDTO, error) {
	if err := a.ensureReady(); err != nil {
		return ProfileDTO{}, err
	}
	if err := a.pinCurrentActiveProfile(); err != nil {
		return ProfileDTO{}, err
	}

	record, err := a.profiles.CreateFromBytes(profile.CreateInput{
		Name:     input.Name,
		Mode:     input.Mode,
		Homepage: input.Homepage,
		BaseURL:  input.BaseURL,
		Tags:     input.Tags,
		Note:     input.Note,
	}, []byte(input.RawJSON))
	if err != nil {
		return ProfileDTO{}, err
	}
	a.notifyStateChanged("profile-imported")
	return mapProfileDetail(record), nil
}

func (a *App) UpdateProfile(input UpdateProfileInput) (ProfileDTO, error) {
	if err := a.ensureReady(); err != nil {
		return ProfileDTO{}, err
	}

	existing, err := a.profiles.Get(input.ID)
	if err != nil {
		return ProfileDTO{}, err
	}
	existing.Name = input.Name
	existing.Mode = input.Mode
	existing.Homepage = input.Homepage
	existing.BaseURL = input.BaseURL
	existing.Tags = input.Tags
	existing.Note = input.Note
	existing.RawJSON = input.RawJSON
	record, err := a.profiles.Update(existing)
	if err != nil {
		return ProfileDTO{}, err
	}
	a.notifyStateChanged("profile-updated")
	return mapProfileDetail(record), nil
}

func (a *App) DeleteProfile(id string) error {
	if err := a.ensureReady(); err != nil {
		return err
	}
	if err := a.profiles.Delete(id); err != nil {
		return err
	}
	settings, err := a.config.Load()
	if err == nil && settings.ActiveProfileID == id {
		settings.ActiveProfileID = ""
		if _, saveErr := a.config.Save(settings); saveErr != nil {
			return saveErr
		}
	}
	a.notifyStateChanged("profile-deleted")
	return nil
}

func (a *App) ReorderProfiles(ids []string) ([]ProfileDTO, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	records, err := a.profiles.Reorder(ids)
	if err != nil {
		return nil, err
	}
	a.notifyStateChanged("profiles-reordered")
	return mapProfiles(records), nil
}

func (a *App) SwitchProfile(profileID string) (SwitchProfileResult, error) {
	if err := a.ensureReady(); err != nil {
		return SwitchProfileResult{}, err
	}
	result, err := a.switchProfileInternal(profileID)
	if err != nil {
		return SwitchProfileResult{}, err
	}
	return mapSwitchResult(result), nil
}

func (a *App) ListAuditLogs() ([]AuditEntryDTO, error) {
	if err := a.ensureReady(); err != nil {
		return nil, err
	}
	entries, err := a.audit.List(100)
	if err != nil {
		return nil, err
	}
	return mapAudits(entries), nil
}

func (a *App) RunDiagnostics() (detector.Diagnostics, error) {
	if err := a.ensureReady(); err != nil {
		return detector.Diagnostics{}, err
	}
	settings, err := a.config.Load()
	if err != nil {
		return detector.Diagnostics{}, err
	}
	return a.detector.RunDiagnostics(settings.TargetAuthPath, settings.ActiveProfileID)
}

func (a *App) switchProfileInternal(profileID string) (switcher.SwitchResult, error) {
	result, err := a.switcher.SwitchProfile(profileID)
	if err != nil {
		return switcher.SwitchResult{}, err
	}
	a.notifyStateChanged("profile-switched")
	return result, nil
}

func (a *App) notifyStateChanged(reason string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "state:changed", reason)
	}
	if a.tray != nil {
		a.tray.Refresh()
	}
}

func (a *App) showMainWindow() {
	if a.ctx == nil {
		return
	}
	runtime.Show(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *App) hideMainWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowHide(a.ctx)
}

func (a *App) quitApplication() {
	if a.ctx == nil {
		return
	}
	runtime.Quit(a.ctx)
}

func (a *App) showTrayError(title, message string) {
	if a.ctx == nil {
		return
	}
	_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.ErrorDialog,
		Title:   title,
		Message: message,
	})
}

func (a *App) pinCurrentActiveProfile() error {
	settings, err := a.config.Load()
	if err != nil {
		return err
	}
	if strings.TrimSpace(settings.ActiveProfileID) != "" || strings.TrimSpace(settings.TargetAuthPath) == "" {
		return nil
	}

	current, err := a.detector.Current(settings.TargetAuthPath, "")
	if err != nil {
		a.writeBestEffortAudit(audit.Entry{
			Action:     "pin_active_profile",
			TargetPath: settings.TargetAuthPath,
			Result:     "warning",
			Message:    fmt.Sprintf("固定当前启用供应商失败: %v", err),
		})
		return nil
	}
	if !current.Managed || strings.TrimSpace(current.ProfileID) == "" {
		return nil
	}

	settings.ActiveProfileID = current.ProfileID
	if _, err := a.config.Save(settings); err != nil {
		a.writeBestEffortAudit(audit.Entry{
			Action:     "pin_active_profile",
			TargetPath: settings.TargetAuthPath,
			Result:     "failed",
			Message:    fmt.Sprintf("保存当前启用供应商失败: %v", err),
		})
		return fmt.Errorf("保存当前启用供应商失败: %w", err)
	}

	return nil
}

func (a *App) writeBestEffortAudit(entry audit.Entry) {
	if a.audit == nil {
		return
	}
	_ = a.audit.Write(entry)
}
