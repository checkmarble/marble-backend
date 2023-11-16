package models

import (
	"fmt"
	"time"
)

type Case struct {
	Id             string
	OrganizationId string
	CreatedAt      time.Time
	Name           string
	Description    string
	Status         CaseStatus
	Decisions      []Decision
	Events         []CaseEvent
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
	Name           string
	Description    string
	OrganizationId string
	DecisionIds    []string
}

type CaseFilters struct {
	StartDate time.Time
	EndDate   time.Time
	Statuses  []CaseStatus
}

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
