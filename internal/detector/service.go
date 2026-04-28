package detector

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codex-profile-manager/internal/codexcfg"
	"codex-profile-manager/internal/profile"
	"codex-profile-manager/internal/util"
)

type CurrentState struct {
	TargetAuthPath string     `json:"targetAuthPath"`
	Exists         bool       `json:"exists"`
	Managed        bool       `json:"managed"`
	ProfileID      string     `json:"profileId,omitempty"`
	ProfileName    string     `json:"profileName,omitempty"`
	Fingerprint    string     `json:"fingerprint,omitempty"`
	Size           int64      `json:"size"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
}

type Diagnostics struct {
	TargetAuthPath    string   `json:"targetAuthPath"`
	TargetDirExists   bool     `json:"targetDirExists"`
	TargetDirWritable bool     `json:"targetDirWritable"`
	AuthFileExists    bool     `json:"authFileExists"`
	AuthFileReadable  bool     `json:"authFileReadable"`
	Managed           bool     `json:"managed"`
	ActiveProfileID   string   `json:"activeProfileId,omitempty"`
	ActiveProfileName string   `json:"activeProfileName,omitempty"`
	ActiveFingerprint string   `json:"activeFingerprint,omitempty"`
	Warnings          []string `json:"warnings"`
}

type Service struct {
	profiles *profile.Service
}

func NewService(profiles *profile.Service) *Service {
	return &Service{
		profiles: profiles,
	}
}

func (s *Service) Current(targetAuthPath, preferredProfileID string) (CurrentState, error) {
	state := CurrentState{
		TargetAuthPath: targetAuthPath,
	}

	fileInfo, err := os.Stat(targetAuthPath)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return CurrentState{}, fmt.Errorf("读取当前 auth.json 状态失败: %w", err)
	}

	payload, err := os.ReadFile(targetAuthPath)
	if err != nil {
		return CurrentState{}, fmt.Errorf("读取当前 auth.json 失败: %w", err)
	}

	normalized, err := util.NormalizeJSON(payload)
	if err != nil {
		normalized = payload
	}

	fingerprint := util.Fingerprint(normalized)
	state.Exists = true
	state.Fingerprint = fingerprint
	state.Size = fileInfo.Size()
	modified := fileInfo.ModTime().UTC()
	state.UpdatedAt = &modified

	configState, err := codexcfg.ReadManagedCustomProvider(codexcfg.ConfigPathForAuthPath(targetAuthPath))
	if err != nil {
		return CurrentState{}, err
	}
	hasUnmanagedCustom, err := codexcfg.HasUnmanagedCustomProvider(codexcfg.ConfigPathForAuthPath(targetAuthPath))
	if err != nil {
		return CurrentState{}, err
	}

	matched, err := s.resolveManagedProfile(fingerprint, preferredProfileID, configState, hasUnmanagedCustom)
	if err != nil {
		return CurrentState{}, err
	}
	if matched != nil {
		state.Managed = true
		state.ProfileID = matched.ID
		state.ProfileName = matched.Name
	}

	return state, nil
}

func (s *Service) RunDiagnostics(targetAuthPath, preferredProfileID string) (Diagnostics, error) {
	current, err := s.Current(targetAuthPath, preferredProfileID)
	if err != nil {
		return Diagnostics{}, err
	}

	dir := filepath.Dir(targetAuthPath)
	diagnostics := Diagnostics{
		TargetAuthPath:    targetAuthPath,
		Managed:           current.Managed,
		ActiveProfileID:   current.ProfileID,
		ActiveProfileName: current.ProfileName,
		ActiveFingerprint: current.Fingerprint,
		Warnings:          []string{},
	}

	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		diagnostics.TargetDirExists = true
		testFile := filepath.Join(dir, ".codex-profile-manager-write-test")
		if err := os.WriteFile(testFile, []byte("ok"), 0o600); err == nil {
			diagnostics.TargetDirWritable = true
			_ = os.Remove(testFile)
		}
	}

	if current.Exists {
		diagnostics.AuthFileExists = true
		if _, err := os.ReadFile(targetAuthPath); err == nil {
			diagnostics.AuthFileReadable = true
		}
	}

	if !diagnostics.TargetDirExists {
		diagnostics.Warnings = append(diagnostics.Warnings, "目标目录不存在")
	}
	if diagnostics.TargetDirExists && !diagnostics.TargetDirWritable {
		diagnostics.Warnings = append(diagnostics.Warnings, "目标目录当前不可写")
	}
	if diagnostics.AuthFileExists && !diagnostics.Managed {
		diagnostics.Warnings = append(diagnostics.Warnings, "当前 auth.json 与已托管资料不匹配")
	}

	return diagnostics, nil
}

func (s *Service) resolveManagedProfile(fingerprint, preferredProfileID string, configState codexcfg.ManagedCustomProvider, hasUnmanagedCustom bool) (*profile.Record, error) {
	if preferredProfileID != "" {
		preferred, err := s.profiles.Get(preferredProfileID)
		if err == nil && profileMatchesCurrent(preferred, fingerprint, configState, hasUnmanagedCustom) {
			return &preferred, nil
		}
	}

	records, err := s.profiles.List()
	if err != nil {
		return nil, err
	}

	var matched *profile.Record
	for _, item := range records {
		if !profileMatchesCurrent(item, fingerprint, configState, hasUnmanagedCustom) {
			continue
		}
		copy := item
		if matched == nil || newerRecord(copy, *matched) {
			matched = &copy
		}
	}

	return matched, nil
}

func profileMatchesCurrent(item profile.Record, fingerprint string, configState codexcfg.ManagedCustomProvider, hasUnmanagedCustom bool) bool {
	if item.Fingerprint != fingerprint {
		return false
	}

	switch item.Mode {
	case profile.ModeAPIKey:
		return configState.Present && strings.TrimSpace(item.BaseURL) == strings.TrimSpace(configState.BaseURL)
	default:
		return !configState.Present && !hasUnmanagedCustom
	}
}

func newerRecord(left, right profile.Record) bool {
	return recordSortTime(left).After(recordSortTime(right))
}

func recordSortTime(item profile.Record) time.Time {
	if item.LastUsedAt != nil {
		return item.LastUsedAt.UTC()
	}
	return item.UpdatedAt.UTC()
}
