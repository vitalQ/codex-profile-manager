package profile_test

import (
	"path/filepath"
	"testing"
	"time"

	"codex-profile-manager/internal/profile"
	"codex-profile-manager/internal/util"
)

func TestCreateAndReadProfile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	service := profile.NewService(filepath.Join(tempDir, "profiles.json"))

	record, err := service.CreateFromBytes(profile.CreateInput{
		Name:     "work-a",
		Homepage: "https://chatgpt.com/codex",
		Tags:     []string{"work", "primary"},
		Note:     "main account",
	}, []byte("{\n  \"token\": \"abc\"\n}"))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}

	if record.Name != "work-a" {
		t.Fatalf("unexpected record name: %s", record.Name)
	}
	if record.Homepage != "https://chatgpt.com/codex" {
		t.Fatalf("unexpected homepage: %s", record.Homepage)
	}

	_, payload, err := service.GetPayload(record.ID)
	if err != nil {
		t.Fatalf("GetPayload() error = %v", err)
	}

	expected, err := util.NormalizeJSON([]byte(`{"token":"abc"}`))
	if err != nil {
		t.Fatalf("NormalizeJSON() error = %v", err)
	}

	if string(payload) != string(expected) {
		t.Fatalf("unexpected payload: %s", string(payload))
	}

	duplicate, err := service.CreateFromBytes(profile.CreateInput{Name: "duplicate"}, []byte(`{"token":"abc"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() duplicate error = %v", err)
	}
	if duplicate.Fingerprint != record.Fingerprint {
		t.Fatalf("expected duplicate fingerprint to match original")
	}
}

func TestReorderProfilesPersistsManualOrder(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	service := profile.NewService(filepath.Join(tempDir, "profiles.json"))

	first, err := service.CreateFromBytes(profile.CreateInput{Name: "first"}, []byte(`{"token":"one"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes(first) error = %v", err)
	}
	second, err := service.CreateFromBytes(profile.CreateInput{Name: "second"}, []byte(`{"token":"two"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes(second) error = %v", err)
	}
	third, err := service.CreateFromBytes(profile.CreateInput{Name: "third"}, []byte(`{"token":"three"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes(third) error = %v", err)
	}

	reordered, err := service.Reorder([]string{third.ID, first.ID, second.ID})
	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}

	if got, want := reordered[0].ID, third.ID; got != want {
		t.Fatalf("reordered[0].ID = %s, want %s", got, want)
	}

	listed, err := service.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	wantOrder := []string{third.ID, first.ID, second.ID}
	for i, want := range wantOrder {
		if listed[i].ID != want {
			t.Fatalf("listed[%d].ID = %s, want %s", i, listed[i].ID, want)
		}
	}
}

func TestFindByFingerprintPrefersMostRecentlyUsedDuplicate(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	service := profile.NewService(filepath.Join(tempDir, "profiles.json"))

	first, err := service.CreateFromBytes(profile.CreateInput{Name: "first"}, []byte(`{"token":"same"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes(first) error = %v", err)
	}
	second, err := service.CreateFromBytes(profile.CreateInput{Name: "second"}, []byte(`{"token":"same"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes(second) error = %v", err)
	}

	if err := service.MarkUsed(second.ID, second.CreatedAt.Add(time.Minute)); err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	match, err := service.FindByFingerprint(first.Fingerprint)
	if err != nil {
		t.Fatalf("FindByFingerprint() error = %v", err)
	}
	if match == nil {
		t.Fatalf("expected a matching profile")
	}
	if match.ID != second.ID {
		t.Fatalf("match.ID = %s, want %s", match.ID, second.ID)
	}
}

func TestCreateAPIKeyProfileValidatesBaseURLAndPayload(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	service := profile.NewService(filepath.Join(tempDir, "profiles.json"))

	record, err := service.CreateFromBytes(profile.CreateInput{
		Name:    "api-key",
		Mode:    profile.ModeAPIKey,
		BaseURL: "https://example.com/v1",
	}, []byte(`{"OPENAI_API_KEY":"sk-demo"}`))
	if err != nil {
		t.Fatalf("CreateFromBytes() error = %v", err)
	}
	if record.Mode != profile.ModeAPIKey {
		t.Fatalf("record.Mode = %s, want %s", record.Mode, profile.ModeAPIKey)
	}
	if record.BaseURL != "https://example.com/v1" {
		t.Fatalf("record.BaseURL = %s", record.BaseURL)
	}

	if _, err := service.CreateFromBytes(profile.CreateInput{
		Name: "invalid-api-key",
		Mode: profile.ModeAPIKey,
	}, []byte(`{"OPENAI_API_KEY":"sk-demo"}`)); err == nil {
		t.Fatalf("expected missing base URL validation error")
	}
}
