package models

import (
	"time"

	"github.com/google/uuid"
)

type AiCaseReview struct {
	ID            uuid.UUID
	CaseID        uuid.UUID
	Status        string
	BucketName    string
	FileReference string
	DtoVersion    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AiCaseReviewStatus int

const (
	AiCaseReviewStatusUnknown AiCaseReviewStatus = iota
	AiCaseReviewStatusPending
	AiCaseReviewStatusCompleted
	AiCaseReviewStatusFailed
)

func (s AiCaseReviewStatus) String() string {
	switch s {
	case AiCaseReviewStatusPending:
		return "pending"
	case AiCaseReviewStatusCompleted:
		return "completed"
	case AiCaseReviewStatusFailed:
		return "failed"
	}
	return "unknown"
}

func AiCaseReviewStatusFromString(s string) AiCaseReviewStatus {
	switch s {
	case "pending":
		return AiCaseReviewStatusPending
	case "completed":
		return AiCaseReviewStatusCompleted
	case "failed":
		return AiCaseReviewStatusFailed
	default:
		return AiCaseReviewStatusUnknown
	}
}
