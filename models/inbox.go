package models

import (
	"time"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type InboxStatus string

const (
	InboxStatusActive   InboxStatus = "active"
	InboxStatusInactive InboxStatus = "archived"
)

type Inbox struct {
	Id                uuid.UUID
	Name              string
	OrganizationId    uuid.UUID
	Status            InboxStatus
	EscalationInboxId *uuid.UUID
	AutoAssignEnabled bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	InboxUsers        []InboxUser
	CasesCount        *int

	// Fields for case review (automatic or manual) settings. May be moved to a separate implementation if or when
	// we have more advanced automations implemented on cases.
	CaseReviewManual        bool
	CaseReviewOnCaseCreated bool
	CaseReviewOnEscalate    bool
}

type InboxMetadata struct {
	Id     uuid.UUID
	Name   string
	Status InboxStatus
}

func (i Inbox) GetMetadata() InboxMetadata {
	return InboxMetadata{
		Id:     i.Id,
		Name:   i.Name,
		Status: i.Status,
	}
}

type CreateInboxInput struct {
	Name              string
	OrganizationId    uuid.UUID
	EscalationInboxId *uuid.UUID
}

type UpdateInboxInput struct {
	Name                    *string                    `json:"name"`
	EscalationInboxId       pure_utils.Null[uuid.UUID] `json:"escalation_inbox_id"`
	AutoAssignEnabled       *bool                      `json:"auto_assign_enabled"`
	CaseReviewManual        *bool                      `json:"case_review_manual"`
	CaseReviewOnCaseCreated *bool                      `json:"case_review_on_case_created"`
	CaseReviewOnEscalate    *bool                      `json:"case_review_on_escalate"`
}
