package models

import (
	"fmt"
	"time"
)

type Case struct {
	Id             string
	Contributors   []CaseContributor
	CreatedAt      time.Time
	Decisions      []Decision
	DecisionsCount int
	Events         []CaseEvent
	InboxId        string
	OrganizationId string
	Name           string
	Status         CaseStatus
	Tags           []CaseTag
	Files          []CaseFile
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
	Id          string
	DecisionIds []string
	InboxId     string
	Name        string
	Status      CaseStatus
}

type CreateCaseCommentAttributes struct {
	Id      string
	Comment string
}

type CaseFilters struct {
	OrganizationId string
	StartDate      time.Time
	EndDate        time.Time
	Statuses       []CaseStatus
	InboxIds       []string
}

type CaseWithRank struct {
	Case
	RankNumber int
	TotalCount TotalCount
}

const (
	CasesSortingCreatedAt SortingField = "created_at"
)

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
