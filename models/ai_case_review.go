package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type AiCaseReviewFeedback struct {
	Reaction *AiCaseReviewReaction
}

type AiCaseReviewStatus int

const (
	AiCaseReviewStatusUnknown AiCaseReviewStatus = iota
	AiCaseReviewStatusPending
	AiCaseReviewStatusCompleted
	AiCaseReviewStatusFailed
	AiCaseReviewStatusInsufficientFunds
)

func (s AiCaseReviewStatus) String() string {
	switch s {
	case AiCaseReviewStatusPending:
		return "pending"
	case AiCaseReviewStatusCompleted:
		return "completed"
	case AiCaseReviewStatusFailed:
		return "failed"
	case AiCaseReviewStatusInsufficientFunds:
		return "insufficient_funds"
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
	case "insufficient_funds":
		return AiCaseReviewStatusInsufficientFunds
	default:
		return AiCaseReviewStatusUnknown
	}
}

type AiCaseReviewReaction string

const (
	AiCaseReviewReactionUnknown AiCaseReviewReaction = "unknown"
	AiCaseReviewReactionOk      AiCaseReviewReaction = "ok"
	AiCaseReviewReactionKo      AiCaseReviewReaction = "ko"
)

func (r AiCaseReviewReaction) String() string {
	switch r {
	case AiCaseReviewReactionOk:
		return "ok"
	case AiCaseReviewReactionKo:
		return "ko"
	}
	return "unknown"
}

func AiCaseReviewReactionFromString(s string) AiCaseReviewReaction {
	switch s {
	case "ok":
		return AiCaseReviewReactionOk
	case "ko":
		return AiCaseReviewReactionKo
	default:
		return AiCaseReviewReactionUnknown
	}
}

type AiCaseReview struct {
	Id                uuid.UUID
	CaseId            uuid.UUID
	Status            AiCaseReviewStatus
	FileReference     string // Reference to final file which contains the case review output
	FileTempReference string // Reference to temporary file which contains data to resume a case review
	BucketName        string
	DtoVersion        string
	CreatedAt         time.Time
	UpdatedAt         time.Time

	AiCaseReviewFeedback
}

// For now, we only support v1 of the case review dto
func NewAiCaseReview(caseId uuid.UUID, bucketName string) AiCaseReview {
	newId := uuid.Must(uuid.NewV7())

	return AiCaseReview{
		Id:                newId,
		CaseId:            caseId,
		Status:            AiCaseReviewStatusPending,
		BucketName:        bucketName,
		FileReference:     fmt.Sprintf("ai_case_reviews/final/%s/%s.json", caseId, newId),
		FileTempReference: fmt.Sprintf("ai_case_reviews/temp/%s/%s.json", caseId, newId),
		DtoVersion:        "v1",
	}
}

type UpdateAiCaseReview struct {
	Status AiCaseReviewStatus
}
