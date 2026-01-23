package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBWebhook struct {
	Id                       uuid.UUID      `db:"id"`
	OrganizationId           uuid.UUID      `db:"organization_id"`
	PartnerId                pgtype.UUID    `db:"partner_id"`
	Name                     pgtype.Text    `db:"name"`
	Url                      string         `db:"url"`
	EventTypes               []string       `db:"event_types"`
	HttpTimeoutSeconds       pgtype.Int4    `db:"http_timeout_seconds"`
	RateLimit                pgtype.Int4    `db:"rate_limit"`
	RateLimitDurationSeconds pgtype.Int4    `db:"rate_limit_duration_seconds"`
	Enabled                  bool           `db:"enabled"`
	CreatedAt                time.Time      `db:"created_at"`
	UpdatedAt                time.Time      `db:"updated_at"`
	DeletedAt                pgtype.Timestamptz `db:"deleted_at"`
}

const TABLE_WEBHOOKS = "webhooks"

var WebhookFields = utils.ColumnList[DBWebhook]()

func AdaptWebhook(db DBWebhook) (models.Webhook, error) {
	webhook := models.Webhook{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		EventTypes:     db.EventTypes,
		Url:            db.Url,
		Enabled:        db.Enabled,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}

	if db.PartnerId.Valid {
		partnerId, _ := uuid.FromBytes(db.PartnerId.Bytes[:])
		webhook.PartnerId = null.StringFrom(partnerId.String())
	}

	if db.Name.Valid {
		webhook.Name = &db.Name.String
	}

	if db.HttpTimeoutSeconds.Valid {
		val := int(db.HttpTimeoutSeconds.Int32)
		webhook.HttpTimeout = &val
	}

	if db.RateLimit.Valid {
		val := int(db.RateLimit.Int32)
		webhook.RateLimit = &val
	}

	if db.RateLimitDurationSeconds.Valid {
		val := int(db.RateLimitDurationSeconds.Int32)
		webhook.RateLimitDuration = &val
	}

	if db.DeletedAt.Valid {
		webhook.DeletedAt = &db.DeletedAt.Time
	}

	return webhook, nil
}

type DBWebhookSecret struct {
	Id          uuid.UUID          `db:"id"`
	WebhookId   uuid.UUID          `db:"webhook_id"`
	SecretValue string             `db:"secret_value"`
	CreatedAt   time.Time          `db:"created_at"`
	ExpiresAt   pgtype.Timestamptz `db:"expires_at"`
	RevokedAt   pgtype.Timestamptz `db:"revoked_at"`
}

const TABLE_WEBHOOK_SECRETS = "webhook_secrets"

var WebhookSecretFields = utils.ColumnList[DBWebhookSecret]()

func AdaptWebhookSecret(db DBWebhookSecret) models.Secret {
	secret := models.Secret{
		Id:        db.Id,
		WebhookId: db.WebhookId,
		Value:     db.SecretValue,
		CreatedAt: db.CreatedAt,
	}

	if db.ExpiresAt.Valid {
		secret.ExpiresAt = &db.ExpiresAt.Time
	}

	if db.RevokedAt.Valid {
		secret.RevokedAt = &db.RevokedAt.Time
	}

	return secret
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
		val := int(db.LastResponseStatus.Int32)
		delivery.LastResponseStatus = &val
	}

	return delivery, nil
}
