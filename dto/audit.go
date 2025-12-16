package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type AuditEventFilters struct {
	OrgId    uuid.UUID  `form:"-"`
	From     *time.Time `form:"from"`
	To       *time.Time `form:"to"`
	UserId   string     `form:"user_id"`
	ApiKeyId string     `form:"api_key_id"`
	Table    string     `form:"table"`
	EntityId string     `form:"entity_id"`
	Limit    int        `form:"limit" binding:"omitempty,gte=1,lte=100"`
	After    string     `form:"after"`
}

type PaginatedAuditEvents struct {
	HasNextPage bool         `json:"has_next_page"`
	Events      []AuditEvent `json:"events"`
}

type AuditEvent struct {
	Id    uuid.UUID       `json:"id"`
	Actor AuditEventActor `json:"actor"`

	EntityId  uuid.UUID       `json:"entity_id"`
	Operation string          `json:"operation"`
	Table     string          `json:"table"`
	OldData   json.RawMessage `json:"old_data"`
	NewData   json.RawMessage `json:"new_data"`

	CreatedAt time.Time `json:"created_at"`
}

type AuditEventActor struct {
	Type string    `json:"type"`
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func AdaptAuditEvent(m models.AuditEvent) AuditEvent {
	return AuditEvent{
		Id: m.Id,
		Actor: AuditEventActor{
			Type: m.Actor.Type,
			Id:   m.Actor.Id,
			Name: m.Actor.Name,
		},
		EntityId:  m.EntityId,
		Operation: m.Operation,
		Table:     m.Table,
		// Let's watch out, we could expose some sensitive data here.
		OldData:   m.OldData,
		NewData:   m.NewData,
		CreatedAt: m.CreatedAt,
	}
}
