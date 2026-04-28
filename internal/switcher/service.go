package switcher

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"codex-profile-manager/internal/audit"
	"codex-profile-manager/internal/codexcfg"
	"codex-profile-manager/internal/codexsession"
	"codex-profile-manager/internal/config"
	"codex-profile-manager/internal/detector"
	"codex-profile-manager/internal/fsx"
	"codex-profile-manager/internal/profile"
	"codex-profile-manager/internal/util"
)

type SwitchResult struct {
	Profile     profile.Record          `json:"profile"`
	Current     detector.CurrentState   `json:"current"`
	SessionSync codexsession.SyncResult `json:"sessionSync"`
}

type Service struct {
	config   *config.Service
	profiles *profile.Service
	audit    *audit.Service
	detector *detector.Service
}

func NewService(
	configService *config.Service,
	profileService *profile.Service,
	auditService *audit.Service,
	detectorService *detector.Service,
) *Service {
	return &Service{
		config:   configService,
		profiles: profileService,
		audit:    auditService,
		detector: detectorService,
	}
}

func (s *Service) SwitchProfile(profileID string) (SwitchResult, error) {
	settings, err := s.config.Load()
	if err != nil {
		return SwitchResult{}, err
	}

	record, payload, err := s.profiles.GetPayload(profileID)
	if err != nil {
		_ = s.audit.Write(audit.Entry{
			Action:     "switch_profile",
			ProfileID:  profileID,
			TargetPath: settings.TargetAuthPath,
			Result:     "failed",
			Message:    err.Error(),
		})
		return SwitchResult{}, err
	}

	sourceProvider, err := currentProvider(settings.TargetAuthPath)
	if err != nil {
		return SwitchResult{}, s.failAudit("switch_profile", record, settings.TargetAuthPath, err)
	}

	sessionSync := codexsession.SyncResult{
		SourceProvider: sourceProvider,
		TargetProvider: codexsession.ProviderForMode(record.Mode),
	}
	if settings.EnableSessionHistorySync && sessionSync.SourceProvider != "" && sessionSync.SourceProvider != sessionSync.TargetProvider {
		sessionSync, err = codexsession.SyncToProvider(settings.TargetAuthPath, sessionSync.TargetProvider)
		if err != nil {
			return SwitchResult{}, s.failAudit("switch_profile", record, settings.TargetAuthPath, err)
		}
		sessionSync.SourceProvider = sourceProvider
	}

	if err := writeTargetAtomically(settings.TargetAuthPath, payload); err != nil {
		return SwitchResult{}, s.failAudit("switch_profile", record, settings.TargetAuthPath, err)
	}

	configPath := codexcfg.ConfigPathForAuthPath(settings.TargetAuthPath)
	if err := syncConfigForProfile(configPath, record); err != nil {
		return SwitchResult{}, s.failAudit("switch_profile", record, settings.TargetAuthPath, err)
	}

	if err := verifyTarget(settings.TargetAuthPath, record.Fingerprint); err != nil {
		return SwitchResult{}, s.failAudit("switch_profile", record, settings.TargetAuthPath, err)
	}
	if err := verifyConfig(configPath, record); err != nil {
		return SwitchResult{}, s.failAudit("switch_profile", record, settings.TargetAuthPath, err)
	}

	if err := s.profiles.MarkUsed(record.ID, time.Now().UTC()); err != nil {
		return SwitchResult{}, err
	}

	settings.ActiveProfileID = record.ID
	if _, err := s.config.Save(settings); err != nil {
		return SwitchResult{}, err
	}

	current, err := s.detector.Current(settings.TargetAuthPath, settings.ActiveProfileID)
	if err != nil {
		return SwitchResult{}, err
	}

	successMessage := "切换成功"
	if sessionSync.Ran && sessionSync.Cloned > 0 {
		successMessage = fmt.Sprintf("切换成功，并同步 %d 条历史会话", sessionSync.Cloned)
	}
	if err := s.audit.Write(audit.Entry{
		Action:      "switch_profile",
		ProfileID:   record.ID,
		ProfileName: record.Name,
		TargetPath:  settings.TargetAuthPath,
		Result:      "success",
		Message:     successMessage,
	}); err != nil {
		return SwitchResult{}, err
	}

	return SwitchResult{
		Profile:     record,
		Current:     current,
		SessionSync: sessionSync,
	}, nil
}

func writeTargetAtomically(targetPath string, payload []byte) error {
	if _, err := util.NormalizeJSON(payload); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(targetPath), "auth.json.tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}

	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.Write(payload); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	if err := fsx.ReplaceFile(tmpPath, targetPath); err != nil {
		return fmt.Errorf("替换目标 auth.json 失败: %w", err)
	}

	return nil
}

func verifyTarget(targetPath, expectedFingerprint string) error {
	payload, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("切换后读取 auth.json 失败: %w", err)
	}

	normalized, err := util.NormalizeJSON(payload)
	if err != nil {
		return err
	}

	if util.Fingerprint(normalized) != expectedFingerprint {
		return fmt.Errorf("切换后校验失败，文件指纹不匹配")
	}

	return nil
}

func syncConfigForProfile(configPath string, record profile.Record) error {
	switch record.Mode {
	case profile.ModeAPIKey:
		if err := codexcfg.EnsureManagedCustomProvider(configPath, record.BaseURL); err != nil {
			return fmt.Errorf("写入 config.toml 失败: %w", err)
		}
	default:
		conflict, err := codexcfg.HasUnmanagedCustomProvider(configPath)
		if err != nil {
			return fmt.Errorf("读取 config.toml 失败: %w", err)
		}
		if conflict {
			return fmt.Errorf("检测到非 Codex Profile Manager 管理的 custom provider 配置，请先手动处理 config.toml")
		}
		if err := codexcfg.RemoveManagedCustomProvider(configPath); err != nil {
			return fmt.Errorf("清理 config.toml 失败: %w", err)
		}
	}
	return nil
}

func verifyConfig(configPath string, record profile.Record) error {
	state, err := codexcfg.ReadManagedCustomProvider(configPath)
	if err != nil {
		return fmt.Errorf("读取 config.toml 失败: %w", err)
	}

	switch record.Mode {
	case profile.ModeAPIKey:
		if !state.Present {
			return fmt.Errorf("切换后校验失败，config.toml 中缺少 custom provider 配置")
		}
		if state.BaseURL != record.BaseURL {
			return fmt.Errorf("切换后校验失败，config.toml 的 Base URL 不匹配")
		}
	default:
		conflict, err := codexcfg.HasUnmanagedCustomProvider(configPath)
		if err != nil {
			return fmt.Errorf("读取 config.toml 失败: %w", err)
		}
		if state.Present {
			return fmt.Errorf("切换后校验失败，config.toml 中仍存在 custom provider 配置")
		}
		if conflict {
			return fmt.Errorf("切换后校验失败，config.toml 中存在非受管 custom provider 配置")
		}
	}
	return nil
}

func currentProvider(targetAuthPath string) (string, error) {
	configPath := codexcfg.ConfigPathForAuthPath(targetAuthPath)
	state, err := codexcfg.ReadManagedCustomProvider(configPath)
	if err != nil {
		return "", fmt.Errorf("读取 config.toml 失败: %w", err)
	}
	hasUnmanagedCustom, err := codexcfg.HasUnmanagedCustomProvider(configPath)
	if err != nil {
		return "", fmt.Errorf("读取 config.toml 失败: %w", err)
	}
	if state.Present || hasUnmanagedCustom {
		return codexsession.ProviderCustom, nil
	}
	return codexsession.ProviderOpenAI, nil
}

func (s *Service) failAudit(action string, record profile.Record, targetPath string, err error) error {
	_ = s.audit.Write(audit.Entry{
		Action:      action,
		ProfileID:   record.ID,
		ProfileName: record.Name,
		TargetPath:  targetPath,
		Result:      "failed",
		Message:     err.Error(),
	})
	return err
}
