package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codex-profile-manager/internal/paths"
	"codex-profile-manager/internal/util"
)

type Settings struct {
	TargetAuthPath           string `json:"targetAuthPath"`
	ActiveProfileID          string `json:"activeProfileId,omitempty"`
	LaunchCodexAfterSwitch   bool   `json:"launchCodexAfterSwitch"`
	AllowPlaintextExport     bool   `json:"allowPlaintextExport"`
	EnableSessionHistorySync bool   `json:"enableSessionHistorySync"`
	Theme                    string `json:"theme"`
}

type Service struct {
	mu       sync.Mutex
	filePath string
	defaults Settings
	cached   *Settings
}

type rawSettings struct {
	TargetAuthPath           string `json:"targetAuthPath"`
	ActiveProfileID          string `json:"activeProfileId,omitempty"`
	LaunchCodexAfterSwitch   bool   `json:"launchCodexAfterSwitch"`
	AllowPlaintextExport     bool   `json:"allowPlaintextExport"`
	EnableSessionHistorySync *bool  `json:"enableSessionHistorySync"`
	Theme                    string `json:"theme"`
}

func NewService(appPaths paths.AppPaths) (*Service, error) {
	targetAuthPath, err := paths.DefaultTargetAuthPath()
	if err != nil {
		return nil, err
	}

	return &Service{
		filePath: appPaths.SettingsFile,
		defaults: Settings{
			TargetAuthPath:           targetAuthPath,
			LaunchCodexAfterSwitch:   false,
			AllowPlaintextExport:     false,
			EnableSessionHistorySync: true,
			Theme:                    "system",
		},
	}, nil
}

func (s *Service) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Service) Save(settings Settings) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := s.applyDefaults(settings)
	if err := s.validate(normalized); err != nil {
		return Settings{}, err
	}

	if err := util.WriteJSONAtomic(s.filePath, normalized); err != nil {
		return Settings{}, fmt.Errorf("保存设置失败: %w", err)
	}
	copy := normalized
	s.cached = &copy

	return normalized, nil
}

func (s *Service) loadLocked() (Settings, error) {
	if s.cached != nil {
		return *s.cached, nil
	}

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		if err := util.WriteJSONAtomic(s.filePath, s.defaults); err != nil {
			return Settings{}, fmt.Errorf("初始化默认设置失败: %w", err)
		}
		copy := s.defaults
		s.cached = &copy
		return s.defaults, nil
	}

	payload, err := os.ReadFile(s.filePath)
	if err != nil {
		return Settings{}, fmt.Errorf("读取设置失败: %w", err)
	}

	var settings rawSettings
	if err := json.Unmarshal(payload, &settings); err != nil {
		return Settings{}, fmt.Errorf("解析设置失败: %w", err)
	}

	normalized := s.applyRawDefaults(settings)
	if err := s.validate(normalized); err != nil {
		return Settings{}, err
	}
	copy := normalized
	s.cached = &copy

	return normalized, nil
}

func (s *Service) applyDefaults(input Settings) Settings {
	result := s.defaults

	if strings.TrimSpace(input.TargetAuthPath) != "" {
		result.TargetAuthPath = strings.TrimSpace(input.TargetAuthPath)
	}
	result.ActiveProfileID = strings.TrimSpace(input.ActiveProfileID)
	result.LaunchCodexAfterSwitch = input.LaunchCodexAfterSwitch
	result.AllowPlaintextExport = input.AllowPlaintextExport
	result.EnableSessionHistorySync = input.EnableSessionHistorySync
	if strings.TrimSpace(input.Theme) != "" {
		result.Theme = strings.TrimSpace(input.Theme)
	}

	return result
}

func (s *Service) applyRawDefaults(input rawSettings) Settings {
	result := s.defaults

	if strings.TrimSpace(input.TargetAuthPath) != "" {
		result.TargetAuthPath = strings.TrimSpace(input.TargetAuthPath)
	}
	result.ActiveProfileID = strings.TrimSpace(input.ActiveProfileID)
	result.LaunchCodexAfterSwitch = input.LaunchCodexAfterSwitch
	result.AllowPlaintextExport = input.AllowPlaintextExport
	if input.EnableSessionHistorySync != nil {
		result.EnableSessionHistorySync = *input.EnableSessionHistorySync
	}
	if strings.TrimSpace(input.Theme) != "" {
		result.Theme = strings.TrimSpace(input.Theme)
	}

	return result
}

func (s *Service) validate(settings Settings) error {
	if strings.TrimSpace(settings.TargetAuthPath) == "" {
		return fmt.Errorf("目标 auth.json 路径不能为空")
	}
	if filepath.Base(settings.TargetAuthPath) == "" {
		return fmt.Errorf("目标 auth.json 路径无效")
	}
	switch settings.Theme {
	case "system", "dark", "light":
	default:
		return fmt.Errorf("主题设置无效")
	}
	return nil
}
