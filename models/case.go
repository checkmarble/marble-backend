package models

import (
	"fmt"
	"time"
)

type Case struct {
	Id             string
	Contributors   []CaseContributor
	CreatedAt      time.Time
	Decisions      []DecisionWithRuleExecutions
	DecisionsCount int
	Events         []CaseEvent
	InboxId        string
	OrganizationId string
	AssignedTo     *UserId
	Name           string
	Status         CaseStatus
	Tags           []CaseTag
	Files          []CaseFile
	SnoozedUntil   *time.Time
}

func (c Case) GetMetadata() CaseMetadata {
	return CaseMetadata{
		Id:             c.Id,
		CreatedAt:      c.CreatedAt,
		OrganizationId: c.OrganizationId,
		Status:         c.Status,
		InboxId:        c.InboxId,
	}
}

func (c Case) IsSnoozed() bool {
	return c.SnoozedUntil != nil && c.SnoozedUntil.After(time.Now())
}

func (c CaseStatus) IsFinalized() bool {
	return c == CaseDiscarded || c == CaseResolved
}

type CaseMetadata struct {
	Id             string
	CreatedAt      time.Time
	OrganizationId string
	Status         CaseStatus
	InboxId        string
}

type CaseStatus string

const (
	CaseOpen          CaseStatus = "open"
	CaseInvestigating CaseStatus = "investigating"
	CaseDiscarded     CaseStatus = "discarded"
	CaseResolved      CaseStatus = "resolved"
	CaseUnknownStatus CaseStatus = "unknown"
)

type CreateCaseAttributes struct {
	DecisionIds    []string
	InboxId        string
	Name           string
	OrganizationId string
}

type UpdateCaseAttributes struct {
	Id      string
	InboxId string
	Name    string
	Status  CaseStatus
}

type CreateCaseCommentAttributes struct {
	Id      string
	Comment string
}

type CaseFilters struct {
	OrganizationId string
	Name           string
	StartDate      time.Time
	EndDate        time.Time
	Statuses       []CaseStatus
	InboxIds       []string
	IncludeSnoozed bool
}

type CaseListPage struct {
	Cases       []Case
	StartIndex  int
	EndIndex    int
	HasNextPage bool
}

type CaseWithRank struct {
	Case
	RankNumber int
}

const CasesSortingCreatedAt = SortingFieldCreatedAt

func ValidateCaseStatuses(statuses []string) ([]CaseStatus, error) {
	sanitizedStatuses := make([]CaseStatus, len(statuses))
	for i, status := range statuses {
		sanitizedStatuses[i] = CaseStatus(status)
		if sanitizedStatuses[i] == CaseUnknownStatus {
			return []CaseStatus{}, fmt.Errorf("invalid status: %s %w", status, BadParameterError)
		}
	}
	return sanitizedStatuses, nil
}

type ReviewCaseDecisionsBody struct {
	DecisionId    string
	ReviewComment string
	ReviewStatus  string
	UserId        string
}

type CaseSnoozeRequest struct {
	UserId UserId
	CaseId string
	Until  time.Time
}

type CaseAssignementRequest struct {
	UserId     UserId
	CaseId     string
	AssigneeId *UserId
}
