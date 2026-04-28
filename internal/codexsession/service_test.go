package codexsession_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codex-profile-manager/internal/codexsession"
)

func TestSyncToProviderClonesCrossProviderSessions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	authPath := filepath.Join(tempDir, ".codex", "auth.json")
	sessionPath := writeSessionFile(t, filepath.Dir(authPath), "2026-04-12T21-59-50", "11111111-1111-1111-1111-111111111111", "openai")

	result, err := codexsession.SyncToProvider(authPath, codexsession.ProviderCustom)
	if err != nil {
		t.Fatalf("SyncToProvider() error = %v", err)
	}
	if !result.Ran {
		t.Fatalf("expected sync to run")
	}
	if result.Cloned != 1 {
		t.Fatalf("result.Cloned = %d, want 1", result.Cloned)
	}

	files := listSessionFiles(t, filepath.Join(filepath.Dir(authPath), "sessions"))
	if len(files) != 2 {
		t.Fatalf("expected 2 rollout files, got %d", len(files))
	}

	var clonePath string
	for _, item := range files {
		if item != sessionPath {
			clonePath = item
		}
	}
	if clonePath == "" {
		t.Fatalf("clone file not found")
	}

	payload := readSessionMetaPayload(t, clonePath)
	if got := payload["model_provider"]; got != codexsession.ProviderCustom {
		t.Fatalf("clone model_provider = %v, want %s", got, codexsession.ProviderCustom)
	}
	if got := payload["cloned_from"]; got != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("clone cloned_from = %v", got)
	}
	if got := payload["root_session_id"]; got != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("clone root_session_id = %v", got)
	}
}

func TestSyncToProviderSkipsSameProviderSessions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	authPath := filepath.Join(tempDir, ".codex", "auth.json")
	writeSessionFile(t, filepath.Dir(authPath), "2026-04-12T21-59-50", "11111111-1111-1111-1111-111111111111", "openai")

	result, err := codexsession.SyncToProvider(authPath, codexsession.ProviderOpenAI)
	if err != nil {
		t.Fatalf("SyncToProvider() error = %v", err)
	}
	if result.Cloned != 0 {
		t.Fatalf("result.Cloned = %d, want 0", result.Cloned)
	}
	if result.SkippedTarget != 1 {
		t.Fatalf("result.SkippedTarget = %d, want 1", result.SkippedTarget)
	}

	files := listSessionFiles(t, filepath.Join(filepath.Dir(authPath), "sessions"))
	if len(files) != 1 {
		t.Fatalf("expected 1 rollout file, got %d", len(files))
	}
}

func TestSyncToProviderDoesNotCloneBackAndForthDuplicates(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	authPath := filepath.Join(tempDir, ".codex", "auth.json")
	writeSessionFile(t, filepath.Dir(authPath), "2026-04-12T21-59-50", "11111111-1111-1111-1111-111111111111", "openai")

	firstResult, err := codexsession.SyncToProvider(authPath, codexsession.ProviderCustom)
	if err != nil {
		t.Fatalf("first SyncToProvider() error = %v", err)
	}
	if firstResult.Cloned != 1 {
		t.Fatalf("firstResult.Cloned = %d, want 1", firstResult.Cloned)
	}

	secondResult, err := codexsession.SyncToProvider(authPath, codexsession.ProviderOpenAI)
	if err != nil {
		t.Fatalf("second SyncToProvider() error = %v", err)
	}
	if secondResult.Cloned != 0 {
		t.Fatalf("secondResult.Cloned = %d, want 0", secondResult.Cloned)
	}
	if secondResult.SkippedExists != 1 {
		t.Fatalf("secondResult.SkippedExists = %d, want 1", secondResult.SkippedExists)
	}
}

func TestSyncToProviderPreservesNonMetaLinesWhenCloning(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	authPath := filepath.Join(tempDir, ".codex", "auth.json")
	sessionPath := writeSessionFile(t, filepath.Dir(authPath), "2026-04-12T21-59-50", "11111111-1111-1111-1111-111111111111", "openai")

	originalRaw, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	result, err := codexsession.SyncToProvider(authPath, codexsession.ProviderCustom)
	if err != nil {
		t.Fatalf("SyncToProvider() error = %v", err)
	}
	if result.Cloned != 1 {
		t.Fatalf("result.Cloned = %d, want 1", result.Cloned)
	}

	files := listSessionFiles(t, filepath.Join(filepath.Dir(authPath), "sessions"))
	if len(files) != 2 {
		t.Fatalf("expected 2 rollout files, got %d", len(files))
	}

	var clonePath string
	for _, item := range files {
		if item != sessionPath {
			clonePath = item
		}
	}
	if clonePath == "" {
		t.Fatalf("clone file not found")
	}

	cloneRaw, err := os.ReadFile(clonePath)
	if err != nil {
		t.Fatalf("ReadFile(clone) error = %v", err)
	}

	originalLines := strings.Split(string(originalRaw), "\n")
	cloneLines := strings.Split(string(cloneRaw), "\n")
	if len(originalLines) != len(cloneLines) {
		t.Fatalf("clone line count = %d, want %d", len(cloneLines), len(originalLines))
	}
	// 除首行 session_meta 会被改写外，其余事件行应保持不变。
	if strings.Join(cloneLines[1:], "\n") != strings.Join(originalLines[1:], "\n") {
		t.Fatalf("expected non-meta lines to be preserved\noriginal=%q\nclone=%q", strings.Join(originalLines[1:], "\n"), strings.Join(cloneLines[1:], "\n"))
	}
}

func TestSyncToProviderOnlyClonesMostRecentTenSessions(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	authPath := filepath.Join(tempDir, ".codex", "auth.json")

	ids := make([]string, 0, 12)
	for index := 1; index <= 12; index++ {
		sessionID := fmt.Sprintf("%012d", index)
		writeSessionFile(
			t,
			filepath.Dir(authPath),
			fmt.Sprintf("2026-04-12T21-%02d-%02d", index/60, index%60),
			sessionID,
			codexsession.ProviderOpenAI,
		)
		ids = append(ids, sessionID)
	}

	result, err := codexsession.SyncToProvider(authPath, codexsession.ProviderCustom)
	if err != nil {
		t.Fatalf("SyncToProvider() error = %v", err)
	}
	if result.Scanned != 10 {
		t.Fatalf("result.Scanned = %d, want 10", result.Scanned)
	}
	if result.Cloned != 10 {
		t.Fatalf("result.Cloned = %d, want 10", result.Cloned)
	}

	files := listSessionFiles(t, filepath.Join(filepath.Dir(authPath), "sessions"))
	if len(files) != 22 {
		t.Fatalf("expected 22 rollout files, got %d", len(files))
	}

	clonedRoots := map[string]bool{}
	for _, path := range files {
		payload := readSessionMetaPayload(t, path)
		if payload["model_provider"] != codexsession.ProviderCustom {
			continue
		}
		rootID, _ := payload["root_session_id"].(string)
		if rootID != "" {
			clonedRoots[rootID] = true
		}
	}

	// 最早的两条历史会话不应进入最近 10 条同步窗口。
	for _, skippedID := range ids[:2] {
		if clonedRoots[skippedID] {
			t.Fatalf("expected oldest session %s to be excluded from sync", skippedID)
		}
	}
	for _, expectedID := range ids[2:] {
		if !clonedRoots[expectedID] {
			t.Fatalf("expected recent session %s to be synced", expectedID)
		}
	}
}

func writeSessionFile(t *testing.T, codexDir, ts, sessionID, provider string) string {
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
	return path
}

func listSessionFiles(t *testing.T, sessionsDir string) []string {
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

func readSessionMetaPayload(t *testing.T, path string) map[string]any {
	t.Helper()

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var first map[string]any
	line := payload[:len(payload)]
	for index, b := range payload {
		if b == '\n' {
			line = payload[:index]
			break
		}
	}

	if err := json.Unmarshal(line, &first); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	metaPayload, _ := first["payload"].(map[string]any)
	return metaPayload
}
