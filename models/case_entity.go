package models

import "github.com/google/uuid"

type CaseEntityRef struct {
	TableName string `json:"table_name"`
	ObjectId  string `json:"object_id"`
}

type CaseManualEntity struct {
	Id             string
	OrganizationId uuid.UUID
	CaseId         string
	CaseEntityRef
}

type CaseEntity struct {
	CaseEntityRef
	Data map[string]any
}
