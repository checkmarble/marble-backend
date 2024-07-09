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

type DBWebhook struct {
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

const TABLE_WEBHOOKS = "webhooks"

var WebhookFields = utils.ColumnList[DBWebhook]()

func AdaptWebhook(db DBWebhook) (models.Webhook, error) {
	eventData := make(map[string]any)
	err := json.Unmarshal(db.EventData, &eventData)
	if err != nil {
		return models.Webhook{}, fmt.Errorf("can't decode %s webhook's event data: %v", db.Id, err)
	}

	return models.Webhook{
		Id:               db.Id,
		CreatedAt:        db.CreatedAt,
		UpdatedAt:        db.UpdatedAt,
		SendAttemptCount: db.SendAttemptCount,
		DeliveryStatus:   models.WebhookDeliveryStatusFrom(db.DeliveryStatus),
		OrganizationId:   db.OrganizationId,
		PartnerId:        null.NewString(db.PartnerId.String, db.PartnerId.Valid),
		EventType:        models.WebhookEventTypeFrom(db.EventType),
		EventData:        eventData,
	}, nil
}
