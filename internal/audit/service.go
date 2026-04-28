package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"codex-profile-manager/internal/util"
)

type Entry struct {
	ID          string    `json:"id"`
	Time        time.Time `json:"time"`
	Action      string    `json:"action"`
	ProfileID   string    `json:"profileId,omitempty"`
	ProfileName string    `json:"profileName,omitempty"`
	TargetPath  string    `json:"targetPath,omitempty"`
	Result      string    `json:"result"`
	Message     string    `json:"message"`
}

type Service struct {
	mu       sync.Mutex
	filePath string
}

func NewService(filePath string) *Service {
	return &Service{filePath: filePath}
}

func (s *Service) Write(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = util.NewID("audit")
	}
	if entry.Time.IsZero() {
		entry.Time = time.Now().UTC()
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(s.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("打开审计日志失败: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("写入审计日志失败: %w", err)
	}

	return nil
}

func (s *Service) List(limit int) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return []Entry{}, nil
	}

	file, err := os.Open(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("读取审计日志失败: %w", err)
	}
	defer file.Close()

	entries := make([]Entry, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("扫描审计日志失败: %w", err)
	}

	for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
		entries[left], entries[right] = entries[right], entries[left]
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}
