package main

import (
	"codex-profile-manager/internal/audit"
	"codex-profile-manager/internal/config"
	"codex-profile-manager/internal/detector"
	"codex-profile-manager/internal/profile"
	"codex-profile-manager/internal/switcher"
)

type ProfileDTO struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Mode        string              `json:"mode"`
	Homepage    string              `json:"homepage"`
	BaseURL     string              `json:"baseUrl,omitempty"`
	Tags        []string            `json:"tags"`
	Note        string              `json:"note"`
	RawJSON     string              `json:"rawJson,omitempty"`
	Fingerprint string              `json:"fingerprint"`
	CreatedAt   string              `json:"createdAt"`
	UpdatedAt   string              `json:"updatedAt"`
	LastUsedAt  string              `json:"lastUsedAt,omitempty"`
	Storage     profile.StorageInfo `json:"storage,omitempty"`
}

type CurrentStateDTO struct {
	TargetAuthPath string `json:"targetAuthPath"`
	Exists         bool   `json:"exists"`
	Managed        bool   `json:"managed"`
	ProfileID      string `json:"profileId,omitempty"`
	ProfileName    string `json:"profileName,omitempty"`
	Fingerprint    string `json:"fingerprint,omitempty"`
	Size           int64  `json:"size"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

type AuditEntryDTO struct {
	ID          string `json:"id"`
	Time        string `json:"time"`
	Action      string `json:"action"`
	ProfileID   string `json:"profileId,omitempty"`
	ProfileName string `json:"profileName,omitempty"`
	TargetPath  string `json:"targetPath,omitempty"`
	Result      string `json:"result"`
	Message     string `json:"message"`
}

type BootstrapData struct {
	Settings config.Settings `json:"settings"`
	Profiles []ProfileDTO    `json:"profiles"`
	Current  CurrentStateDTO `json:"current"`
}

type ImportProfileInput struct {
	Name     string   `json:"name"`
	Mode     string   `json:"mode"`
	Homepage string   `json:"homepage"`
	BaseURL  string   `json:"baseUrl"`
	Tags     []string `json:"tags"`
	Note     string   `json:"note"`
}

type ImportProfileFromFileInput struct {
	Name     string   `json:"name"`
	Mode     string   `json:"mode"`
	Homepage string   `json:"homepage"`
	BaseURL  string   `json:"baseUrl"`
	Tags     []string `json:"tags"`
	Note     string   `json:"note"`
	FilePath string   `json:"filePath"`
}

type ImportProfileFromRawInput struct {
	Name     string   `json:"name"`
	Mode     string   `json:"mode"`
	Homepage string   `json:"homepage"`
	BaseURL  string   `json:"baseUrl"`
	Tags     []string `json:"tags"`
	Note     string   `json:"note"`
	RawJSON  string   `json:"rawJson"`
}

type UpdateProfileInput struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Mode     string   `json:"mode"`
	Homepage string   `json:"homepage"`
	BaseURL  string   `json:"baseUrl"`
	Tags     []string `json:"tags"`
	Note     string   `json:"note"`
	RawJSON  string   `json:"rawJson"`
}

type SwitchProfileResult struct {
	Profile     ProfileDTO      `json:"profile"`
	Current     CurrentStateDTO `json:"current"`
	SessionSync SessionSyncDTO  `json:"sessionSync"`
}

type SessionSyncDTO struct {
	Ran            bool   `json:"ran"`
	SourceProvider string `json:"sourceProvider,omitempty"`
	TargetProvider string `json:"targetProvider,omitempty"`
	Scanned        int    `json:"scanned"`
	Cloned         int    `json:"cloned"`
	SkippedExists  int    `json:"skippedExists"`
	SkippedTarget  int    `json:"skippedTarget"`
	SkippedInvalid int    `json:"skippedInvalid"`
}

func mapProfileSummary(record profile.Record) ProfileDTO {
	result := ProfileDTO{
		ID:          record.ID,
		Name:        record.Name,
		Mode:        record.Mode,
		Homepage:    record.Homepage,
		BaseURL:     record.BaseURL,
		Tags:        record.Tags,
		Note:        record.Note,
		Fingerprint: record.Fingerprint,
		CreatedAt:   record.CreatedAt.Format(timeLayout),
		UpdatedAt:   record.UpdatedAt.Format(timeLayout),
	}
	if record.LastUsedAt != nil {
		result.LastUsedAt = record.LastUsedAt.Format(timeLayout)
	}
	return result
}

func mapProfileDetail(record profile.Record) ProfileDTO {
	result := mapProfileSummary(record)
	result.RawJSON = record.RawJSON
	result.Storage = record.Storage
	return result
}

func mapProfiles(records []profile.Record) []ProfileDTO {
	result := make([]ProfileDTO, 0, len(records))
	for _, record := range records {
		result = append(result, mapProfileSummary(record))
	}
	return result
}

func mapCurrent(state detector.CurrentState) CurrentStateDTO {
	result := CurrentStateDTO{
		TargetAuthPath: state.TargetAuthPath,
		Exists:         state.Exists,
		Managed:        state.Managed,
		ProfileID:      state.ProfileID,
		ProfileName:    state.ProfileName,
		Fingerprint:    state.Fingerprint,
		Size:           state.Size,
	}
	if state.UpdatedAt != nil {
		result.UpdatedAt = state.UpdatedAt.Format(timeLayout)
	}
	return result
}

func mapAudit(entry audit.Entry) AuditEntryDTO {
	return AuditEntryDTO{
		ID:          entry.ID,
		Time:        entry.Time.Format(timeLayout),
		Action:      entry.Action,
		ProfileID:   entry.ProfileID,
		ProfileName: entry.ProfileName,
		TargetPath:  entry.TargetPath,
		Result:      entry.Result,
		Message:     entry.Message,
	}
}

func mapAudits(entries []audit.Entry) []AuditEntryDTO {
	result := make([]AuditEntryDTO, 0, len(entries))
	for _, entry := range entries {
		result = append(result, mapAudit(entry))
	}
	return result
}

func mapSwitchResult(result switcher.SwitchResult) SwitchProfileResult {
	return SwitchProfileResult{
		Profile: mapProfileSummary(result.Profile),
		Current: mapCurrent(result.Current),
		SessionSync: SessionSyncDTO{
			Ran:            result.SessionSync.Ran,
			SourceProvider: result.SessionSync.SourceProvider,
			TargetProvider: result.SessionSync.TargetProvider,
			Scanned:        result.SessionSync.Scanned,
			Cloned:         result.SessionSync.Cloned,
			SkippedExists:  result.SessionSync.SkippedExists,
			SkippedTarget:  result.SessionSync.SkippedTarget,
			SkippedInvalid: result.SessionSync.SkippedInvalid,
		},
	}
}

const timeLayout = "2006-01-02T15:04:05Z07:00"
