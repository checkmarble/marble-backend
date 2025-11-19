package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbAuditEvent struct {
	Id       uuid.UUID `db:"id"`
	EntityId uuid.UUID `db:"entity_id"`

	UserId   *uuid.UUID `db:"user_id"`
	ApiKeyId *uuid.UUID `db:"api_key_id"`

	Operation    string          `db:"operation"`
	Table        string          `db:"table"`
	PreviousData json.RawMessage `db:"previous_data"`
	Data         json.RawMessage `db:"data"`

	CreatedAt time.Time `db:"created_at"`
}

type DbAuditEventWithActor struct {
	DbAuditEvent

	UserName   *string `db:"user_name"`
	ApiKeyName *string `db:"api_key_name"`
}

const TABLE_AUDIT_EVENTS = "audit.audit_events"

var SelectAuditEventColumns = utils.EscapedColumnList[DbAuditEvent]()

func AdaptAuditEvent(db DbAuditEvent) (models.AuditEvent, error) {
	return models.AuditEvent{
		Id:        db.Id,
		EntityId:  db.EntityId,
		Operation: db.Operation,
		Table:     db.Table,
		OldData:   db.PreviousData,
		NewData:   db.Data,
		CreatedAt: db.CreatedAt,
	}, nil
}

func AdaptAuditEventWithActor(db DbAuditEventWithActor) (models.AuditEvent, error) {
	event, _ := AdaptAuditEvent(db.DbAuditEvent)

	switch {
	case db.UserId != nil:
		event.Actor = models.AuditEventActor{
			Type: "user",
			Id:   *db.UserId,
			Name: pure_utils.PtrValueOrDefault(db.UserName, "n/a"),
		}

	case db.ApiKeyId != nil:
		event.Actor = models.AuditEventActor{
			Type: "api_key",
			Id:   *db.ApiKeyId,
			Name: pure_utils.PtrValueOrDefault(db.ApiKeyName, "n/a"),
		}
	}

	return event, nil
}
