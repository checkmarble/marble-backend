package models

import "time"

type Case struct {
	Id             string
	OrganizationId string
	CreatedAt 		time.Time
	Name           string
	Description    *string
	Status				 CaseStatus
}

type CaseStatus string

const (
	CaseOpen CaseStatus = "open"
	CaseInvestigating CaseStatus = "investigating"
	CaseDiscarded CaseStatus = "discarded"
	CaseResolved CaseStatus = "resolved"
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
	return CaseOpen
}
