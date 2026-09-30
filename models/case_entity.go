package models

import "github.com/google/uuid"

// CaseEntityRef is the canonical identity of an ingested object within a case's organization.
type CaseEntityRef struct {
	TableName string
	ObjectId  string
}

type CaseManualEntity struct {
	Id             string
	OrganizationId uuid.UUID
	CaseId         string
	CaseEntityRef
}

type CaseEntitySource string

const (
	CaseEntitySourceManual   CaseEntitySource = "manual"
	CaseEntitySourceDecision CaseEntitySource = "decision"
)

type CaseEntity struct {
	CaseEntityRef
	Sources []CaseEntitySource
	Data    map[string]any
}
