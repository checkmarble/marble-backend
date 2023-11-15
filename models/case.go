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
}

type CaseStatus string

const (
	CaseOpen          CaseStatus = "open"
	CaseInvestigating CaseStatus = "investigating"
	CaseDiscarded     CaseStatus = "discarded"
	CaseResolved      CaseStatus = "resolved"
	CaseUnknownStatus CaseStatus = "unknown"
)

func CaseStatusFrom(s string) CaseStatus {
	switch s {
	case "open":
		return CaseOpen
	case "investigating":
		return CaseInvestigating
	case "discarded":
		return CaseDiscarded
	case "resolved":
		return CaseResolved
	}
	return CaseUnknownStatus
}

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
		sanitizedStatuses[i] = CaseStatusFrom(status)
		if sanitizedStatuses[i] == CaseUnknownStatus {
			return []CaseStatus{}, fmt.Errorf("invalid status: %s %w", status, BadParameterError)
		}
	}
	return sanitizedStatuses, nil
}
