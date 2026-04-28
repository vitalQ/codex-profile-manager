package codexsession

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"codex-profile-manager/internal/fsx"

	"github.com/google/uuid"
)

const (
	ProviderOpenAI = "openai"
	ProviderCustom = "custom"
	// 为了控制同步成本，只处理最近 10 条有效会话。
	maxRecentSyncs = 10
)

var rolloutFilePattern = regexp.MustCompile(`^rollout-(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2})-[0-9a-fA-F-]+\.jsonl$`)

var errInvalidSessionFile = errors.New("invalid session file")

type SyncResult struct {
	Ran            bool   `json:"ran"`
	SourceProvider string `json:"sourceProvider,omitempty"`
	TargetProvider string `json:"targetProvider,omitempty"`
	Scanned        int    `json:"scanned"`
	Cloned         int    `json:"cloned"`
	SkippedExists  int    `json:"skippedExists"`
	SkippedTarget  int    `json:"skippedTarget"`
	SkippedInvalid int    `json:"skippedInvalid"`
}

type fileCloneResult struct {
	action string
}

// sessionDescriptor 缓存会话文件里会反复用到的元信息，避免重复读盘。
type sessionDescriptor struct {
	path     string
	provider string
	key      string
	valid    bool
}

func ProviderForMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "api_key":
		return ProviderCustom
	default:
		return ProviderOpenAI
	}
}

func SyncToProvider(targetAuthPath, targetProvider string) (SyncResult, error) {
	result := SyncResult{
		Ran:            true,
		TargetProvider: strings.TrimSpace(targetProvider),
	}
	if result.TargetProvider == "" {
		return result, fmt.Errorf("目标 provider 不能为空")
	}

	sessionsDir := filepath.Join(filepath.Dir(targetAuthPath), "sessions")
	sessionFiles, err := listSessionFiles(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	}

	// 先统一解析元信息，后面的去重和“最近会话”筛选都复用这份结果。
	descriptors := describeSessionFiles(sessionFiles)
	existingKeys := buildCloneIndex(descriptors, result.TargetProvider)
	for _, sessionFile := range selectRecentSyncFiles(descriptors, maxRecentSyncs) {
		result.Scanned++

		cloneResult, err := cloneSessionFile(sessionFile, sessionsDir, result.TargetProvider, existingKeys)
		if err != nil {
			return result, err
		}

		switch cloneResult.action {
		case "cloned":
			result.Cloned++
		case "skipped_exists":
			result.SkippedExists++
		case "skipped_target":
			result.SkippedTarget++
		case "skipped_invalid":
			result.SkippedInvalid++
		}
	}

	return result, nil
}

func listSessionFiles(sessionsDir string) ([]string, error) {
	result := []string{}
	if _, err := os.Stat(sessionsDir); err != nil {
		return nil, err
	}

	err := filepath.WalkDir(sessionsDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		result = append(result, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(result)
	return result, nil
}

func describeSessionFiles(sessionFiles []string) []sessionDescriptor {
	descriptors := make([]sessionDescriptor, 0, len(sessionFiles))
	for _, sessionFile := range sessionFiles {
		payload, err := readSessionMetaPayload(sessionFile)
		if err != nil {
			descriptors = append(descriptors, sessionDescriptor{path: sessionFile})
			continue
		}
		descriptors = append(descriptors, sessionDescriptor{
			path:     sessionFile,
			provider: stringField(payload["model_provider"]),
			key:      dedupeKey(payload),
			valid:    true,
		})
	}
	return descriptors
}

func buildCloneIndex(descriptors []sessionDescriptor, targetProvider string) map[string]struct{} {
	index := make(map[string]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		if !descriptor.valid || descriptor.provider != targetProvider || descriptor.key == "" {
			continue
		}
		index[descriptor.key] = struct{}{}
	}
	return index
}

func selectRecentSyncFiles(descriptors []sessionDescriptor, limit int) []string {
	if limit <= 0 {
		return nil
	}

	// 描述列表已按文件名排序；从尾部回扫即可拿到时间上最近的有效会话。
	selected := make([]string, 0, min(limit, len(descriptors)))
	for index := len(descriptors) - 1; index >= 0 && len(selected) < limit; index-- {
		descriptor := descriptors[index]
		if !descriptor.valid || descriptor.provider == "" {
			continue
		}
		selected = append(selected, descriptor.path)
	}

	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}
	return selected
}

func cloneSessionFile(sessionFile, sessionsDir, targetProvider string, existingKeys map[string]struct{}) (fileCloneResult, error) {
	metaPayload, err := readSessionMetaPayload(sessionFile)
	if err != nil {
		return fileCloneResult{action: "skipped_invalid"}, nil
	}

	currentProvider := stringField(metaPayload["model_provider"])
	if currentProvider == targetProvider {
		return fileCloneResult{action: "skipped_target"}, nil
	}

	currentID := stringField(metaPayload["id"])
	if currentID == "" {
		return fileCloneResult{action: "skipped_invalid"}, nil
	}

	key := dedupeKey(metaPayload)
	if key == "" {
		key = currentID
	}
	if _, exists := existingKeys[key]; exists {
		return fileCloneResult{action: "skipped_exists"}, nil
	}

	newPayload := copyMap(metaPayload)
	newPayload["id"] = uuid.NewString()
	newPayload["model_provider"] = targetProvider
	newPayload["cloned_from"] = currentID
	newPayload["root_session_id"] = key
	if currentProvider != "" {
		newPayload["original_provider"] = currentProvider
	}
	newPayload["clone_timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	targetPath := buildClonePath(sessionsDir, filepath.Base(sessionFile), stringField(newPayload["id"]))

	if _, err := os.Stat(targetPath); err == nil {
		return fileCloneResult{action: "skipped_exists"}, nil
	} else if !os.IsNotExist(err) {
		return fileCloneResult{}, fmt.Errorf("检查克隆目标失败: %w", err)
	}

	if err := cloneSessionWithUpdatedMeta(sessionFile, targetPath, newPayload); err != nil {
		if errors.Is(err, errInvalidSessionFile) {
			return fileCloneResult{action: "skipped_invalid"}, nil
		}
		return fileCloneResult{}, fmt.Errorf("写入克隆 session 失败: %w", err)
	}

	existingKeys[key] = struct{}{}
	return fileCloneResult{action: "cloned"}, nil
}

func buildClonePath(sessionsDir, originalName, newID string) string {
	timestampToken := time.Now().UTC().Format("2006-01-02T15-04-05")
	if matches := rolloutFilePattern.FindStringSubmatch(originalName); len(matches) > 1 {
		timestampToken = matches[1]
	}

	dateToken := strings.SplitN(timestampToken, "T", 2)[0]
	parts := strings.Split(dateToken, "-")
	if len(parts) != 3 {
		parts = []string{
			time.Now().UTC().Format("2006"),
			time.Now().UTC().Format("01"),
			time.Now().UTC().Format("02"),
		}
	}

	return filepath.Join(
		sessionsDir,
		parts[0],
		parts[1],
		parts[2],
		fmt.Sprintf("rollout-%s-%s.jsonl", timestampToken, newID),
	)
}

func readSessionMetaPayload(path string) (map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return nil, readErr
		}

		stripped := strings.TrimSpace(line)
		if stripped != "" {
			var obj map[string]any
			if err := json.Unmarshal([]byte(stripped), &obj); err != nil {
				return nil, err
			}
			if obj["type"] == "session_meta" {
				payload, ok := obj["payload"].(map[string]any)
				if !ok {
					return nil, fmt.Errorf("session_meta payload 无效")
				}
				return payload, nil
			}
		}

		if readErr == io.EOF {
			break
		}
	}

	return nil, fmt.Errorf("未找到 session_meta")
}

func cloneSessionWithUpdatedMeta(sourcePath, targetPath string, newPayload map[string]any) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(targetPath), filepath.Base(targetPath)+".tmp-*")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()
	success := false
	defer func() {
		_ = tmpFile.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	// 逐行复制原始 JSONL，只替换 session_meta，保证其余事件内容原样保留。
	reader := bufio.NewReader(sourceFile)
	replaced := false
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		if readErr == io.EOF && line == "" {
			break
		}

		outputLine := line
		if stripped := strings.TrimSpace(line); stripped != "" {
			var obj map[string]any
			if err := json.Unmarshal([]byte(stripped), &obj); err != nil {
				return errInvalidSessionFile
			}
			if !replaced && obj["type"] == "session_meta" {
				updated := copyMap(obj)
				updated["payload"] = newPayload

				encoded, err := json.Marshal(updated)
				if err != nil {
					return fmt.Errorf("序列化克隆后的 session_meta 失败: %w", err)
				}
				outputLine = string(encoded) + trailingNewline(line)
				replaced = true
			}
		}

		if _, err := tmpFile.WriteString(outputLine); err != nil {
			return err
		}

		if readErr == io.EOF {
			break
		}
	}

	if !replaced {
		return errInvalidSessionFile
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := fsx.ReplaceFile(tmpPath, targetPath); err != nil {
		return err
	}

	success = true
	return nil
}

func dedupeKey(payload map[string]any) string {
	for _, field := range []string{"root_session_id", "cloned_from", "id"} {
		if value := stringField(payload[field]); value != "" {
			return value
		}
	}
	return ""
}

func copyMap(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func stringField(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func trailingNewline(line string) string {
	if strings.HasSuffix(line, "\n") {
		return "\n"
	}
	return ""
}
