package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// DB models for new webhook delivery system

type DBNewWebhook struct {
	Id                       uuid.UUID    `db:"id"`
	OrganizationId           uuid.UUID    `db:"organization_id"`
	Url                      string       `db:"url"`
	EventTypes               []string     `db:"event_types"`
	HttpTimeoutSeconds       pgtype.Int4  `db:"http_timeout_seconds"`
	RateLimit                pgtype.Int4  `db:"rate_limit"`
	RateLimitDurationSeconds pgtype.Int4  `db:"rate_limit_duration_seconds"`
	Enabled                  bool         `db:"enabled"`
	CreatedAt                time.Time    `db:"created_at"`
	UpdatedAt                time.Time    `db:"updated_at"`
	DeletedAt                pgtype.Timestamptz `db:"deleted_at"`
}

const TABLE_NEW_WEBHOOKS = "webhooks"

var NewWebhookFields = utils.ColumnList[DBNewWebhook]()

func AdaptNewWebhook(db DBNewWebhook) (models.NewWebhook, error) {
	webhook := models.NewWebhook{
		Id:                 db.Id,
		OrganizationId:     db.OrganizationId,
		Url:                db.Url,
		EventTypes:         db.EventTypes,
		HttpTimeoutSeconds: 30, // default
		Enabled:            db.Enabled,
		CreatedAt:          db.CreatedAt,
		UpdatedAt:          db.UpdatedAt,
	}

	if db.HttpTimeoutSeconds.Valid {
		webhook.HttpTimeoutSeconds = int(db.HttpTimeoutSeconds.Int32)
	}
	if db.RateLimit.Valid {
		webhook.RateLimit = utils.Ptr(int(db.RateLimit.Int32))
	}
	if db.RateLimitDurationSeconds.Valid {
		webhook.RateLimitDurationSeconds = utils.Ptr(int(db.RateLimitDurationSeconds.Int32))
	}
	if db.DeletedAt.Valid {
		webhook.DeletedAt = &db.DeletedAt.Time
	}

	return webhook, nil
}

type DBNewWebhookSecret struct {
	Id        uuid.UUID          `db:"id"`
	WebhookId uuid.UUID          `db:"webhook_id"`
	Value     string             `db:"secret_value"`
	CreatedAt time.Time          `db:"created_at"`
	ExpiresAt pgtype.Timestamptz `db:"expires_at"`
	RevokedAt pgtype.Timestamptz `db:"revoked_at"`
}

const TABLE_WEBHOOK_SECRETS = "webhook_secrets"

var WebhookSecretFields = utils.ColumnList[DBNewWebhookSecret]()

func AdaptNewWebhookSecret(db DBNewWebhookSecret) (models.NewWebhookSecret, error) {
	secret := models.NewWebhookSecret{
		Id:        db.Id,
		WebhookId: db.WebhookId,
		Value:     db.Value,
		CreatedAt: db.CreatedAt,
	}

	if db.ExpiresAt.Valid {
		secret.ExpiresAt = &db.ExpiresAt.Time
	}
	if db.RevokedAt.Valid {
		secret.RevokedAt = &db.RevokedAt.Time
	}

	return secret, nil
}

type DBWebhookEventV2 struct {
	Id             uuid.UUID `db:"id"`
	OrganizationId uuid.UUID `db:"organization_id"`
	EventType      string    `db:"event_type"`
	EventData      []byte    `db:"event_data"`
	CreatedAt      time.Time `db:"created_at"`
}

const TABLE_WEBHOOK_EVENTS_V2 = "webhook_events_v2"

var WebhookEventV2Fields = utils.ColumnList[DBWebhookEventV2]()

func AdaptWebhookEventV2(db DBWebhookEventV2) (models.WebhookEventV2, error) {
	return models.WebhookEventV2{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		EventType:      db.EventType,
		EventData:      db.EventData,
		CreatedAt:      db.CreatedAt,
	}, nil
}

type DBWebhookDelivery struct {
	Id                 uuid.UUID          `db:"id"`
	WebhookEventId     uuid.UUID          `db:"webhook_event_id"`
	WebhookId          uuid.UUID          `db:"webhook_id"`
	Status             string             `db:"status"`
	Attempts           int                `db:"attempts"`
	NextRetryAt        pgtype.Timestamptz `db:"next_retry_at"`
	LastError          pgtype.Text        `db:"last_error"`
	LastResponseStatus pgtype.Int4        `db:"last_response_status"`
	CreatedAt          time.Time          `db:"created_at"`
	UpdatedAt          time.Time          `db:"updated_at"`
}

const TABLE_WEBHOOK_DELIVERIES = "webhook_deliveries"

var WebhookDeliveryFields = utils.ColumnList[DBWebhookDelivery]()

func AdaptWebhookDelivery(db DBWebhookDelivery) (models.WebhookDelivery, error) {
	delivery := models.WebhookDelivery{
		Id:             db.Id,
		WebhookEventId: db.WebhookEventId,
		WebhookId:      db.WebhookId,
		Status:         models.WebhookDeliveryStatus(db.Status),
		Attempts:       db.Attempts,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}

	if db.NextRetryAt.Valid {
		delivery.NextRetryAt = &db.NextRetryAt.Time
	}
	if db.LastError.Valid {
		delivery.LastError = &db.LastError.String
	}
	if db.LastResponseStatus.Valid {
		delivery.LastResponseStatus = utils.Ptr(int(db.LastResponseStatus.Int32))
	}

	return delivery, nil
}
