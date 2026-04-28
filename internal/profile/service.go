package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"codex-profile-manager/internal/util"
)

type StorageInfo struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type Record struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Mode        string      `json:"mode"`
	Homepage    string      `json:"homepage"`
	BaseURL     string      `json:"baseUrl,omitempty"`
	Tags        []string    `json:"tags"`
	Note        string      `json:"note"`
	RawJSON     string      `json:"rawJson"`
	Fingerprint string      `json:"fingerprint"`
	SortIndex   int         `json:"sortIndex"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	LastUsedAt  *time.Time  `json:"lastUsedAt,omitempty"`
	Storage     StorageInfo `json:"storage"`
}

type indexFile struct {
	Profiles []Record `json:"profiles"`
}

type CreateInput struct {
	Name     string   `json:"name"`
	Mode     string   `json:"mode"`
	Homepage string   `json:"homepage"`
	BaseURL  string   `json:"baseUrl"`
	Tags     []string `json:"tags"`
	Note     string   `json:"note"`
}

const (
	ModeOfficial = "official"
	ModeAPIKey   = "api_key"
)

type Service struct {
	mu        sync.Mutex
	indexPath string
	cached    *indexFile
}

func NewService(indexPath string) *Service {
	return &Service{
		indexPath: indexPath,
	}
}

func (s *Service) List() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndexLocked()
	if err != nil {
		return nil, err
	}

	sort.SliceStable(index.Profiles, func(i, j int) bool {
		return index.Profiles[i].SortIndex < index.Profiles[j].SortIndex
	})

	return index.Profiles, nil
}

func (s *Service) CreateFromBytes(input CreateInput, payload []byte) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Record{}, fmt.Errorf("资料名称不能为空")
	}

	mode, err := normalizeMode(input.Mode)
	if err != nil {
		return Record{}, err
	}

	normalized, err := util.NormalizeJSON(payload)
	if err != nil {
		return Record{}, err
	}
	if err := validateModePayload(mode, normalized, input.BaseURL); err != nil {
		return Record{}, err
	}
	fingerprint := util.Fingerprint(normalized)

	index, err := s.loadIndexLocked()
	if err != nil {
		return Record{}, err
	}

	now := time.Now().UTC()
	id := util.NewID("profile")

	record := Record{
		ID:          id,
		Name:        name,
		Mode:        mode,
		Homepage:    strings.TrimSpace(input.Homepage),
		BaseURL:     normalizeBaseURL(mode, input.BaseURL),
		Tags:        normalizeTags(input.Tags),
		Note:        strings.TrimSpace(input.Note),
		RawJSON:     string(normalized),
		Fingerprint: fingerprint,
		SortIndex:   len(index.Profiles),
		CreatedAt:   now,
		UpdatedAt:   now,
		Storage: StorageInfo{
			Type: "inline",
			Path: s.indexPath,
		},
	}

	index.Profiles = append(index.Profiles, record)
	if err := s.saveIndexLocked(index); err != nil {
		return Record{}, err
	}

	return record, nil
}

func (s *Service) Update(input Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Record{}, fmt.Errorf("资料名称不能为空")
	}

	mode, err := normalizeMode(input.Mode)
	if err != nil {
		return Record{}, err
	}

	normalized, err := util.NormalizeJSON([]byte(input.RawJSON))
	if err != nil {
		return Record{}, err
	}
	if err := validateModePayload(mode, normalized, input.BaseURL); err != nil {
		return Record{}, err
	}
	fingerprint := util.Fingerprint(normalized)

	index, err := s.loadIndexLocked()
	if err != nil {
		return Record{}, err
	}

	for i := range index.Profiles {
		if index.Profiles[i].ID == input.ID {
			index.Profiles[i].Name = name
			index.Profiles[i].Mode = mode
			index.Profiles[i].Homepage = strings.TrimSpace(input.Homepage)
			index.Profiles[i].BaseURL = normalizeBaseURL(mode, input.BaseURL)
			index.Profiles[i].Tags = normalizeTags(input.Tags)
			index.Profiles[i].Note = strings.TrimSpace(input.Note)
			index.Profiles[i].RawJSON = string(normalized)
			index.Profiles[i].Fingerprint = fingerprint
			index.Profiles[i].UpdatedAt = time.Now().UTC()
			if err := s.saveIndexLocked(index); err != nil {
				return Record{}, err
			}
			return index.Profiles[i], nil
		}
	}

	return Record{}, fmt.Errorf("未找到资料: %s", input.ID)
}

func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndexLocked()
	if err != nil {
		return err
	}

	filtered := make([]Record, 0, len(index.Profiles))
	var target *Record
	for i := range index.Profiles {
		record := index.Profiles[i]
		if record.ID == id {
			target = &record
			continue
		}
		filtered = append(filtered, record)
	}

	if target == nil {
		return fmt.Errorf("未找到资料: %s", id)
	}

	index.Profiles = filtered
	reindexProfiles(index.Profiles)
	return s.saveIndexLocked(index)
}

func (s *Service) Get(id string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.findByIDLocked(id)
}

func (s *Service) GetPayload(id string) (Record, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, err := s.findByIDLocked(id)
	if err != nil {
		return Record{}, nil, err
	}

	return record, []byte(record.RawJSON), nil
}

func (s *Service) MarkUsed(id string, usedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndexLocked()
	if err != nil {
		return err
	}

	for i := range index.Profiles {
		if index.Profiles[i].ID == id {
			ts := usedAt.UTC()
			index.Profiles[i].LastUsedAt = &ts
			index.Profiles[i].UpdatedAt = ts
			return s.saveIndexLocked(index)
		}
	}

	return fmt.Errorf("未找到资料: %s", id)
}

func (s *Service) Reorder(ids []string) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndexLocked()
	if err != nil {
		return nil, err
	}
	if len(ids) != len(index.Profiles) {
		return nil, fmt.Errorf("排序请求与现有资料数量不一致")
	}

	recordsByID := make(map[string]Record, len(index.Profiles))
	for _, item := range index.Profiles {
		recordsByID[item.ID] = item
	}

	reordered := make([]Record, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		record, ok := recordsByID[id]
		if !ok {
			return nil, fmt.Errorf("排序请求包含未知资料: %s", id)
		}
		if _, exists := seen[id]; exists {
			return nil, fmt.Errorf("排序请求包含重复资料: %s", id)
		}
		seen[id] = struct{}{}
		reordered = append(reordered, record)
	}

	reindexProfiles(reordered)
	index.Profiles = reordered
	if err := s.saveIndexLocked(index); err != nil {
		return nil, err
	}
	return index.Profiles, nil
}

func (s *Service) FindByFingerprint(fingerprint string) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index, err := s.loadIndexLocked()
	if err != nil {
		return nil, err
	}

	var matched *Record
	for _, item := range index.Profiles {
		if item.Fingerprint == fingerprint {
			copy := item
			if matched == nil || newerRecord(copy, *matched) {
				matched = &copy
			}
		}
	}
	return matched, nil
}

func (s *Service) findByIDLocked(id string) (Record, error) {
	index, err := s.loadIndexLocked()
	if err != nil {
		return Record{}, err
	}

	for _, item := range index.Profiles {
		if item.ID == id {
			return item, nil
		}
	}

	return Record{}, fmt.Errorf("未找到资料: %s", id)
}

func (s *Service) loadIndexLocked() (indexFile, error) {
	if s.cached != nil {
		return cloneIndex(*s.cached), nil
	}

	if _, err := os.Stat(s.indexPath); os.IsNotExist(err) {
		initial := indexFile{Profiles: []Record{}}
		if err := s.saveIndexLocked(initial); err != nil {
			return indexFile{}, err
		}
		return initial, nil
	}

	payload, err := os.ReadFile(s.indexPath)
	if err != nil {
		return indexFile{}, fmt.Errorf("读取资料索引失败: %w", err)
	}

	var index indexFile
	if err := json.Unmarshal(payload, &index); err != nil {
		return indexFile{}, fmt.Errorf("解析资料索引失败: %w", err)
	}

	if index.Profiles == nil {
		index.Profiles = []Record{}
	}
	reindexProfiles(index.Profiles)
	cached := cloneIndex(index)
	s.cached = &cached

	return cloneIndex(index), nil
}

func (s *Service) saveIndexLocked(index indexFile) error {
	if err := util.WriteJSONAtomic(s.indexPath, index); err != nil {
		return err
	}
	cached := cloneIndex(index)
	s.cached = &cached
	return nil
}

func normalizeTags(input []string) []string {
	if len(input) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))
	for _, item := range input {
		tag := strings.TrimSpace(item)
		if tag == "" {
			continue
		}
		lower := strings.ToLower(tag)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		result = append(result, tag)
	}

	sort.Strings(result)
	return result
}

func reindexProfiles(records []Record) {
	for i := range records {
		records[i].SortIndex = i
	}
}

func newerRecord(left, right Record) bool {
	return recordSortTime(left).After(recordSortTime(right))
}

func recordSortTime(item Record) time.Time {
	if item.LastUsedAt != nil {
		return item.LastUsedAt.UTC()
	}
	return item.UpdatedAt.UTC()
}

func cloneIndex(input indexFile) indexFile {
	return indexFile{
		Profiles: cloneRecords(input.Profiles),
	}
}

func cloneRecords(input []Record) []Record {
	if len(input) == 0 {
		return []Record{}
	}

	result := make([]Record, len(input))
	for i, item := range input {
		copy := item
		copy.Mode = defaultMode(copy.Mode)
		if item.Tags != nil {
			copy.Tags = append([]string(nil), item.Tags...)
		} else {
			copy.Tags = []string{}
		}
		if item.LastUsedAt != nil {
			ts := item.LastUsedAt.UTC()
			copy.LastUsedAt = &ts
		}
		result[i] = copy
	}
	return result
}

func normalizeMode(mode string) (string, error) {
	mode = defaultMode(mode)
	switch mode {
	case ModeOfficial, ModeAPIKey:
		return mode, nil
	default:
		return "", fmt.Errorf("资料模式无效")
	}
}

func defaultMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return ModeOfficial
	}
	return mode
}

func normalizeBaseURL(mode, baseURL string) string {
	if mode != ModeAPIKey {
		return ""
	}
	return strings.TrimSpace(baseURL)
}

func validateModePayload(mode string, payload []byte, baseURL string) error {
	if mode != ModeAPIKey {
		return nil
	}
	if strings.TrimSpace(baseURL) == "" {
		return fmt.Errorf("API Key 模式下 Base URL 不能为空")
	}

	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return fmt.Errorf("无效的 JSON: %w", err)
	}

	value, ok := parsed["OPENAI_API_KEY"]
	if !ok {
		return fmt.Errorf("API Key 模式下 auth.json 必须包含 OPENAI_API_KEY")
	}
	key, _ := value.(string)
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("OPENAI_API_KEY 不能为空")
	}
	return nil
}
