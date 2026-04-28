package codexcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"codex-profile-manager/internal/util"
)

const (
	StartMarker = "# codex-profile-manager:start"
	EndMarker   = "# codex-profile-manager:end"
)

var (
	reModelProviderCustom = regexp.MustCompile(`(?m)^\s*model_provider\s*=\s*"custom"\s*$`)
	reCustomSection       = regexp.MustCompile(`(?m)^\s*\[model_providers\.custom\]\s*$`)
	reBaseURL             = regexp.MustCompile(`(?m)^\s*base_url\s*=\s*"([^"]*)"\s*$`)
)

type ManagedCustomProvider struct {
	Present bool
	BaseURL string
}

func HasUnmanagedCustomProvider(path string) (bool, error) {
	content, err := readConfig(path)
	if err != nil {
		return false, err
	}
	return hasUnmanagedCustomProvider(content), nil
}

func ConfigPathForAuthPath(authPath string) string {
	if strings.TrimSpace(authPath) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(authPath), "config.toml")
}

func ReadManagedCustomProvider(path string) (ManagedCustomProvider, error) {
	content, err := readConfig(path)
	if err != nil {
		return ManagedCustomProvider{}, err
	}
	if strings.TrimSpace(content) == "" {
		return ManagedCustomProvider{}, nil
	}

	start, end, ok, err := managedBlockBounds(content)
	if err != nil {
		return ManagedCustomProvider{}, err
	}
	if !ok {
		return ManagedCustomProvider{}, nil
	}

	block := content[start:end]
	baseURL := ""
	if matches := reBaseURL.FindStringSubmatch(block); len(matches) > 1 {
		baseURL = strings.TrimSpace(matches[1])
	}

	return ManagedCustomProvider{
		Present: true,
		BaseURL: baseURL,
	}, nil
}

func EnsureManagedCustomProvider(path, baseURL string) error {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return fmt.Errorf("Base URL 不能为空")
	}

	content, err := readConfig(path)
	if err != nil {
		return err
	}

	start, end, ok, err := managedBlockBounds(content)
	if err != nil {
		return err
	}
	if hasUnmanagedCustomProvider(content) {
		return fmt.Errorf("检测到非 Codex Profile Manager 管理的 custom provider 配置，请先手动处理 config.toml")
	}

	content = stripManagedModelProviderLine(content)
	start, end, ok, err = managedBlockBounds(content)
	if err != nil {
		return err
	}

	block := renderManagedBlock(baseURL)
	if ok {
		content = content[:start] + block + content[end:]
	} else {
		content = appendManagedBlock(content, block)
	}

	content = prependManagedModelProviderLine(content)
	return util.WriteFileAtomic(path, []byte(strings.TrimRight(content, "\n")+"\n"))
}

func RemoveManagedCustomProvider(path string) error {
	content, err := readConfig(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		return nil
	}

	start, end, ok, err := managedBlockBounds(content)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	content = stripManagedModelProviderLine(content)
	start, end, ok, err = managedBlockBounds(content)
	if err != nil {
		return err
	}
	if !ok {
		updated := collapseBlankLines(content)
		if strings.TrimSpace(updated) == "" {
			return util.WriteFileAtomic(path, []byte{})
		}
		return util.WriteFileAtomic(path, []byte(strings.TrimRight(updated, "\n")+"\n"))
	}

	updated := collapseBlankLines(content[:start] + content[end:])
	if strings.TrimSpace(updated) == "" {
		return util.WriteFileAtomic(path, []byte{})
	}
	return util.WriteFileAtomic(path, []byte(strings.TrimRight(updated, "\n")+"\n"))
}

func readConfig(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 config.toml 失败: %w", err)
	}
	return strings.ReplaceAll(string(payload), "\r\n", "\n"), nil
}

func renderManagedBlock(baseURL string) string {
	return strings.Join([]string{
		StartMarker,
		`[model_providers.custom]`,
		`name = "custom"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		fmt.Sprintf(`base_url = %q`, baseURL),
		EndMarker,
		"",
	}, "\n")
}

func managedBlockBounds(content string) (int, int, bool, error) {
	start := strings.Index(content, StartMarker)
	end := strings.Index(content, EndMarker)

	if start == -1 && end == -1 {
		return 0, 0, false, nil
	}
	if start == -1 || end == -1 || end < start {
		return 0, 0, false, fmt.Errorf("config.toml 中的 Codex Profile Manager 受管配置块不完整")
	}

	end += len(EndMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return start, end, true, nil
}

func hasUnmanagedCustomProvider(content string) bool {
	start, end, ok, err := managedBlockBounds(content)
	if err != nil {
		return true
	}
	outside := content
	if ok {
		outside = content[:start] + content[end:]
		outside = stripManagedModelProviderLine(outside)
	}
	return reModelProviderCustom.MatchString(outside) || reCustomSection.MatchString(outside)
}

func prependManagedModelProviderLine(content string) string {
	content = strings.TrimLeft(content, "\n")
	line := `model_provider = "custom"`
	if content == "" {
		return line + "\n"
	}
	return line + "\n" + content
}

func stripManagedModelProviderLine(content string) string {
	if strings.HasPrefix(content, "model_provider = \"custom\"\n") {
		return strings.TrimLeft(strings.TrimPrefix(content, "model_provider = \"custom\"\n"), "\n")
	}
	if content == `model_provider = "custom"` {
		return ""
	}
	return content
}

func appendManagedBlock(content, block string) string {
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return block
	}
	return trimmed + "\n\n" + block
}

func collapseBlankLines(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	blankCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount > 1 {
				continue
			}
			result = append(result, "")
			continue
		}
		blankCount = 0
		result = append(result, line)
	}
	return strings.Trim(strings.Join(result, "\n"), "\n")
}
