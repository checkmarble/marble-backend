package dbmodels

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/guregu/null/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBWebhookEvent struct {
	Id               string      `db:"id"`
	CreatedAt        time.Time   `db:"created_at"`
	UpdatedAt        time.Time   `db:"updated_at"`
	SendAttemptCount int         `db:"send_attempt_count"`
	DeliveryStatus   string      `db:"delivery_status"`
	OrganizationId   string      `db:"organization_id"`
	PartnerId        pgtype.Text `db:"partner_id"`
	EventType        string      `db:"event_type"`
	EventData        []byte      `db:"event_data"`
}

const TABLE_WEBHOOK_EVENTS = "webhook_events"

var WebhookEventFields = utils.ColumnList[DBWebhookEvent]()

func AdaptWebhookEvent(db DBWebhookEvent) (models.WebhookEvent, error) {
	eventData := make(map[string]any)
	err := json.Unmarshal(db.EventData, &eventData)
	if err != nil {
		return models.WebhookEvent{}, fmt.Errorf("can't decode %s webhook's event data: %v", db.Id, err)
	}

	return models.WebhookEvent{
		Id:               db.Id,
		CreatedAt:        db.CreatedAt,
		UpdatedAt:        db.UpdatedAt,
		SendAttemptCount: db.SendAttemptCount,
		DeliveryStatus:   models.WebhookEventDeliveryStatus(db.DeliveryStatus),
		OrganizationId:   db.OrganizationId,
		PartnerId:        null.NewString(db.PartnerId.String, db.PartnerId.Valid),
		EventContent: models.WebhookEventContent{
			Type: models.WebhookEventType(db.EventType),
			Data: eventData,
		},
	}, nil
}
