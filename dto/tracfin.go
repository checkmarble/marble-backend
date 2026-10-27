package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"time"
)

// Pointer revisions distinguish an omitted guard from explicit creation at zero.
type TracfinRevisionRequest struct {
	ExpectedRevision *int64 `json:"expected_revision"`
}
type TracfinConfigurationRequest struct {
	ExpectedRevision *int64                       `json:"expected_revision"`
	Settings         []models.TracfinSettingInput `json:"settings"`
	Mappings         []models.TracfinMapping      `json:"mappings"`
}
type TracfinCreateDraftRequest struct {
	DeclarantIsActor *bool `json:"declarant_is_actor"`
}
type TracfinDraftRequest struct {
	ExpectedRevision *int64                      `json:"expected_revision"`
	Changes          []models.TracfinChangeInput `json:"changes"`
}
type TracfinNavigateRequest struct {
	Selection models.TracfinSelection `json:"selection"`
	StepID    string                  `json:"step_id"`
	Cursor    *string                 `json:"cursor,omitempty"`
	Limit     *int                    `json:"limit,omitempty"`
}

// Export storage locations are internal; downloads always pass through case authorization.
type TracfinExport struct {
	ID             string                   `json:"id"`
	DraftRevision  int64                    `json:"draft_revision"`
	ProfileVersion string                   `json:"profile_version"`
	SHA256         string                   `json:"sha256"`
	Size           int64                    `json:"size"`
	Validation     models.TracfinValidation `json:"validation"`
	CreatedAt      time.Time                `json:"created_at"`
	DownloadURL    string                   `json:"download_url"`
}

func AdaptTracfinExport(e models.TracfinExport, caseID string) TracfinExport {
	return TracfinExport{ID: e.ID, DraftRevision: e.DraftRevision, ProfileVersion: e.ProfileVersion, SHA256: e.SHA256, Size: e.Size, Validation: e.Validation, CreatedAt: e.CreatedAt, DownloadURL: "/cases/" + caseID + "/tracfin/exports/" + e.ID}
}

// Draft responses retain the stable editing model while keeping storage proof private.
type TracfinAttachment struct {
	SourceFileID string `json:"source_file_id"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
}
type TracfinBlock struct {
	models.TracfinBlock
	Children   []TracfinBlock     `json:"children,omitempty"`
	Attachment *TracfinAttachment `json:"attachment,omitempty"`
}
type TracfinDocument struct {
	Root TracfinBlock `json:"root"`
}
type TracfinDraft struct {
	models.TracfinDraft
	Document   TracfinDocument `json:"document"`
	LastExport *TracfinExport  `json:"last_export,omitempty"`
}

func adaptTracfinBlock(b models.TracfinBlock) TracfinBlock {
	out := TracfinBlock{TracfinBlock: b}
	for _, child := range b.Children {
		out.Children = append(out.Children, adaptTracfinBlock(child))
	}
	if b.Attachment != nil {
		out.Attachment = &TracfinAttachment{SourceFileID: b.Attachment.SourceFileID, SHA256: b.Attachment.SHA256, Size: b.Attachment.Size}
	}
	return out
}
func AdaptTracfinDraft(d models.TracfinDraft) TracfinDraft {
	out := TracfinDraft{TracfinDraft: d, Document: TracfinDocument{Root: adaptTracfinBlock(d.Document.Root)}}
	if d.LastExport != nil {
		e := AdaptTracfinExport(*d.LastExport, d.CaseID)
		out.LastExport = &e
	}
	return out
}
