package models

import (
	"time"

	"github.com/google/uuid"
)

type AiCaseReviewFile struct {
	ID            uuid.UUID
	CaseID        uuid.UUID
	Status        string
	BucketName    string
	FileReference string
	DtoVersion    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AiCaseReviewFileStatus int

const (
	AiCaseReviewFileStatusUnknown AiCaseReviewFileStatus = iota
	AiCaseReviewFileStatusPending
	AiCaseReviewFileStatusCompleted
	AiCaseReviewFileStatusFailed
)

func (s AiCaseReviewFileStatus) String() string {
	switch s {
	case AiCaseReviewFileStatusPending:
		return "pending"
	case AiCaseReviewFileStatusCompleted:
		return "completed"
	case AiCaseReviewFileStatusFailed:
		return "failed"
	}
	return "unknown"
}

func AiCaseReviewFileStatusFromString(s string) AiCaseReviewFileStatus {
	switch s {
	case "pending":
		return AiCaseReviewFileStatusPending
	case "completed":
		return AiCaseReviewFileStatusCompleted
	case "failed":
		return AiCaseReviewFileStatusFailed
	default:
		return AiCaseReviewFileStatusUnknown
	}
}
