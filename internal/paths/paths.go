package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const AppDirName = "CodexProfileManager"

type AppPaths struct {
	BaseDir           string `json:"baseDir"`
	ProfilesDir       string `json:"profilesDir"`
	LogsDir           string `json:"logsDir"`
	SettingsFile      string `json:"settingsFile"`
	ProfilesIndexFile string `json:"profilesIndexFile"`
	AuditLogFile      string `json:"auditLogFile"`
}

func Resolve() (AppPaths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return AppPaths{}, fmt.Errorf("无法获取用户配置目录: %w", err)
	}

	baseDir := filepath.Join(configDir, AppDirName)

	result := AppPaths{
		BaseDir:           baseDir,
		ProfilesDir:       filepath.Join(baseDir, "profiles"),
		LogsDir:           filepath.Join(baseDir, "logs"),
		SettingsFile:      filepath.Join(baseDir, "settings.json"),
		ProfilesIndexFile: filepath.Join(baseDir, "profiles.json"),
		AuditLogFile:      filepath.Join(baseDir, "logs", "audit.jsonl"),
	}

	for _, dir := range []string{
		result.BaseDir,
		result.ProfilesDir,
		result.LogsDir,
	} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return AppPaths{}, fmt.Errorf("无法创建目录 %s: %w", dir, err)
		}
	}

	return result, nil
}

func DefaultTargetAuthPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户目录: %w", err)
	}

	return filepath.Join(homeDir, ".codex", "auth.json"), nil
}
