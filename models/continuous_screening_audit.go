package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ContinuousScreeningAuditAction int

const (
	ContinuousScreeningAuditActionUnknown ContinuousScreeningAuditAction = iota
	ContinuousScreeningAuditActionAdd
	ContinuousScreeningAuditActionRemove
)

func (a ContinuousScreeningAuditAction) String() string {
	switch a {
	case ContinuousScreeningAuditActionAdd:
		return "add"
	case ContinuousScreeningAuditActionRemove:
		return "remove"
	default:
		return "unknown"
	}
}

func ContinuousScreeningAuditActionFrom(s string) ContinuousScreeningAuditAction {
	switch s {
	case "add":
		return ContinuousScreeningAuditActionAdd
	case "remove":
		return ContinuousScreeningAuditActionRemove
	default:
		return ContinuousScreeningAuditActionUnknown
	}
}

type ContinuousScreeningAudit struct {
	Id             uuid.UUID
	ObjectType     string
	ObjectId       string
	ConfigStableId uuid.UUID
	Action         ContinuousScreeningAuditAction
	UserId         *uuid.UUID
	ApiKeyId       *uuid.UUID
	CreatedAt      time.Time
	Extra          json.RawMessage
}

type CreateContinuousScreeningAudit struct {
	ObjectType     string
	ObjectId       string
	ConfigStableId uuid.UUID
	Action         ContinuousScreeningAuditAction
	UserId         *uuid.UUID
	ApiKeyId       *uuid.UUID
	Extra          json.RawMessage
}
