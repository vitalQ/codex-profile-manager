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
	reAPIKeySection      = regexp.MustCompile(`(?m)^\s*\[model_providers\.(OpenAI)\]\s*$`)
	reAnyProviderSection = regexp.MustCompile(`(?m)^\s*\[model_providers\.([^\]]+)\]\s*$`)
	reBaseURL            = regexp.MustCompile(`(?m)^\s*base_url\s*=\s*"([^"]*)"\s*$`)
	reSupportsWebSockets = regexp.MustCompile(`(?m)^\s*supports_websockets\s*=\s*true\s*$`)
	reModelProviderLine  = regexp.MustCompile(`^(\s*model_provider\s*=\s*)"([^"]*)"(.*)$`)
)

type ManagedCustomProvider struct {
	Present            bool
	BaseURL            string
	SupportsWebSockets bool
	Provider           string
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
	supportsWebSockets := reSupportsWebSockets.MatchString(block)
	provider := ""
	if matches := reAPIKeySection.FindStringSubmatch(block); len(matches) > 1 {
		provider = strings.TrimSpace(matches[1])
	}

	return ManagedCustomProvider{
		Present:            true,
		BaseURL:            baseURL,
		SupportsWebSockets: supportsWebSockets,
		Provider:           provider,
	}, nil
}

func EnsureManagedCustomProvider(path, baseURL string, supportsWebSockets bool) error {
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
	if ok && !managedBlockUsesOpenAI(content[start:end]) {
		return fmt.Errorf("检测到非 OpenAI 的 Codex Profile Manager 受管 provider 配置，请先手动处理 config.toml")
	}
	if hasUnmanagedCustomProvider(content) {
		return fmt.Errorf("检测到非 Codex Profile Manager 管理的 OpenAI provider 配置，请先手动处理 config.toml")
	}

	// Extract user-added extra lines from existing managed block before replacing.
	var extraLines []string
	if ok {
		extraLines = ParseExtraLines(content[start:end])
	}

	block := renderManagedBlock(baseURL, supportsWebSockets, extraLines)
	if ok {
		content = content[:start] + block + content[end:]
	} else {
		content = appendManagedBlock(content, block)
	}
	content = normalizeExistingModelProvider(content, "OpenAI")

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

	updated := collapseBlankLines(content[:start] + content[end:])
	updated = normalizeExistingModelProvider(updated, "openai")
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

func renderManagedBlock(baseURL string, supportsWebSockets bool, extraLines []string) string {
	lines := []string{
		StartMarker,
		`[model_providers.OpenAI]`,
		`name = "OpenAI"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
	}
	if supportsWebSockets {
		lines = append(lines, `supports_websockets = true`)
	}
	lines = append(lines, fmt.Sprintf(`base_url = %q`, baseURL))
	lines = append(lines, extraLines...)
	lines = append(lines, EndMarker, "")
	return strings.Join(lines, "\n")
}

// managedKeys are the keys that renderManagedBlock always generates.
// parseExtraLines excludes these so they don't get duplicated.
var managedKeys = map[string]bool{
	"name":                 true,
	"wire_api":             true,
	"requires_openai_auth": true,
	"supports_websockets":  true,
	"base_url":             true,
}

// ParseExtraLines extracts user-added lines from an existing managed block,
// excluding markers, the section header, blank lines, and known managed keys.
func ParseExtraLines(block string) []string {
	var extra []string
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip markers and section header.
		if trimmed == StartMarker || trimmed == EndMarker {
			continue
		}
		if trimmed == "[model_providers.OpenAI]" {
			continue
		}
		// Check if this line is a known managed key (key = value format).
		if eqIdx := strings.Index(trimmed, "="); eqIdx > 0 {
			key := strings.TrimSpace(trimmed[:eqIdx])
			if managedKeys[key] {
				continue
			}
		}
		extra = append(extra, line)
	}
	return extra
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
	}
	return reAPIKeySection.MatchString(outside)
}

func managedBlockUsesOpenAI(block string) bool {
	for _, match := range reAnyProviderSection.FindAllStringSubmatch(block, -1) {
		provider := strings.TrimSpace(match[1])
		if provider != "" && provider != "OpenAI" {
			return false
		}
	}
	return true
}

func normalizeExistingModelProvider(content, provider string) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			break
		}
		matches := reModelProviderLine.FindStringSubmatch(line)
		if len(matches) == 0 {
			continue
		}
		if matches[2] != provider {
			lines[index] = matches[1] + fmt.Sprintf("%q", provider) + matches[3]
		}
		break
	}
	return strings.Join(lines, "\n")
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
