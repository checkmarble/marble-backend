package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type InboxDto struct {
	Id                uuid.UUID      `json:"id"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	Name              string         `json:"name"`
	Status            string         `json:"status"`
	EscalationInboxId *uuid.UUID     `json:"escalation_inbox_id"`
	AutoAssignEnabled bool           `json:"auto_assign_enabled"`
	Users             []InboxUserDto `json:"users"`
	Sla               *int           `json:"sla"`

	CaseReviewManual        bool `json:"case_review_manual"`
	CaseReviewOnCaseCreated bool `json:"case_review_on_case_created"`
	CaseReviewOnEscalate    bool `json:"case_review_on_escalate"`

	CasesCount *int `json:"cases_count"`
}

func AdaptInboxDto(i models.Inbox) InboxDto {
	return InboxDto{
		Id:                      i.Id,
		CreatedAt:               i.CreatedAt,
		UpdatedAt:               i.UpdatedAt,
		Name:                    i.Name,
		Status:                  string(i.Status),
		EscalationInboxId:       i.EscalationInboxId,
		AutoAssignEnabled:       i.AutoAssignEnabled,
		Users:                   pure_utils.Map(i.InboxUsers, AdaptInboxUserDto),
		Sla:                     i.Sla,
		CaseReviewManual:        i.CaseReviewManual,
		CaseReviewOnCaseCreated: i.CaseReviewOnCaseCreated,
		CaseReviewOnEscalate:    i.CaseReviewOnEscalate,
		CasesCount:              i.CasesCount,
	}
}

type InboxUserDto struct {
	Id             uuid.UUID `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Role           string    `json:"role"`
	AutoAssignable bool      `json:"auto_assignable"`
	InboxId        uuid.UUID `json:"inbox_id"`
	UserId         uuid.UUID `json:"user_id"`
}

func AdaptInboxUserDto(i models.InboxUser) InboxUserDto {
	return InboxUserDto{
		Id:             i.Id,
		CreatedAt:      i.CreatedAt,
		UpdatedAt:      i.UpdatedAt,
		Role:           string(i.Role),
		AutoAssignable: i.AutoAssignable,
		InboxId:        i.InboxId,
		UserId:         i.UserId,
	}
}

type InboxMetadataDto struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func AdaptInboxMetadataDto(i models.InboxMetadata) InboxMetadataDto {
	return InboxMetadataDto{
		Id:   i.Id,
		Name: i.Name,
	}
}

type UpdateInboxInput struct {
	Name                    *string                    `json:"name"`
	EscalationInboxId       pure_utils.Null[uuid.UUID] `json:"escalation_inbox_id"`
	AutoAssignEnabled       *bool                      `json:"auto_assign_enabled"`
	CaseReviewManual        *bool                      `json:"case_review_manual"`
	CaseReviewOnCaseCreated *bool                      `json:"case_review_on_case_created"`
	CaseReviewOnEscalate    *bool                      `json:"case_review_on_escalate"`
	Sla                     pure_utils.Null[int]       `json:"sla" binding:"omitempty,min=1"`
}

func AdaptUpdateInboxInput(i UpdateInboxInput) models.UpdateInboxInput {
	return models.UpdateInboxInput{
		Name:                    i.Name,
		EscalationInboxId:       i.EscalationInboxId,
		AutoAssignEnabled:       i.AutoAssignEnabled,
		CaseReviewManual:        i.CaseReviewManual,
		CaseReviewOnCaseCreated: i.CaseReviewOnCaseCreated,
		CaseReviewOnEscalate:    i.CaseReviewOnEscalate,
		Sla:                     i.Sla,
	}
}

type UpdateInboxItem struct {
	Id  uuid.UUID `json:"id" binding:"required"`
	Sla *int      `json:"sla" binding:"omitempty,min=1"`
}

type BulkUpdateInboxInput struct {
	Inboxes []UpdateInboxItem `json:"inboxes" binding:"required"`
}

func AdaptBulkUpdateInboxInput(i BulkUpdateInboxInput) []models.UpdateInboxItem {
	return pure_utils.Map(i.Inboxes, func(item UpdateInboxItem) models.UpdateInboxItem {
		return models.UpdateInboxItem{
			Id:  item.Id,
			Sla: item.Sla,
		}
	})
}
