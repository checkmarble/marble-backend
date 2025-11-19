package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuditEvent struct {
	Id    uuid.UUID
	Actor AuditEventActor

	EntityId  uuid.UUID
	Operation string
	Table     string
	OldData   json.RawMessage
	NewData   json.RawMessage

	CreatedAt time.Time
}

type AuditEventActor struct {
	Type string
	Id   uuid.UUID
	Name string
}
