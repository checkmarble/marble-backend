package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type WebhookRepository interface {
	// Webhook CRUD
	CreateWebhook(ctx context.Context, exec Executor, webhook models.NewWebhook) error
	GetWebhook(ctx context.Context, exec Executor, id uuid.UUID) (models.NewWebhook, error)
	ListWebhooks(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.NewWebhook, error)
	ListWebhooksByEventType(ctx context.Context, exec Executor, orgId uuid.UUID, eventType string) ([]models.NewWebhook, error)
	UpdateWebhook(ctx context.Context, exec Executor, id uuid.UUID, update models.NewWebhookUpdate) error
	DeleteWebhook(ctx context.Context, exec Executor, id uuid.UUID) error

	// Secret management
	AddSecret(ctx context.Context, exec Executor, secret models.NewWebhookSecret) error
	ListActiveSecrets(ctx context.Context, exec Executor, webhookId uuid.UUID) ([]models.NewWebhookSecret, error)
	RevokeSecret(ctx context.Context, exec Executor, secretId uuid.UUID) error

	// Queue management
	CreateWebhookQueueItem(ctx context.Context, exec Executor, item models.WebhookQueueItem) error
	GetWebhookQueueItem(ctx context.Context, exec Executor, id uuid.UUID) (models.WebhookQueueItem, error)

	// Delivery tracking
	CreateDelivery(ctx context.Context, exec Executor, delivery models.WebhookDelivery) error
	GetDelivery(ctx context.Context, exec Executor, id uuid.UUID) (models.WebhookDelivery, error)
	DeliveryExists(ctx context.Context, exec Executor, webhookEventId, webhookId uuid.UUID) (bool, error)
	UpdateDeliverySuccess(ctx context.Context, exec Executor, id uuid.UUID, responseStatus int) error
	UpdateDeliveryFailed(ctx context.Context, exec Executor, id uuid.UUID, errMsg string, responseStatus *int) error
	UpdateDeliveryAttempt(ctx context.Context, exec Executor, id uuid.UUID, errMsg string, responseStatus *int, attempts int, nextRetryAt time.Time) error
}

type WebhookRepositoryPostgresql struct{}

func (repo *WebhookRepositoryPostgresql) CreateWebhook(ctx context.Context, exec Executor, webhook models.NewWebhook) error {
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_NEW_WEBHOOKS).
		Columns(
			"id",
			"organization_id",
			"url",
			"event_types",
			"http_timeout_seconds",
			"rate_limit",
			"rate_limit_duration_seconds",
			"enabled",
		).
		Values(
			webhook.Id,
			webhook.OrganizationId,
			webhook.Url,
			webhook.EventTypes,
			webhook.HttpTimeoutSeconds,
			webhook.RateLimit,
			webhook.RateLimitDurationSeconds,
			webhook.Enabled,
		)

	return ExecBuilder(ctx, exec, query)
}

func (repo *WebhookRepositoryPostgresql) GetWebhook(ctx context.Context, exec Executor, id uuid.UUID) (models.NewWebhook, error) {
	query := NewQueryBuilder().
		Select(dbmodels.NewWebhookFields...).
		From(dbmodels.TABLE_NEW_WEBHOOKS).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"deleted_at": nil})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptNewWebhook)
}

func (repo *WebhookRepositoryPostgresql) ListWebhooks(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.NewWebhook, error) {
	query := NewQueryBuilder().
		Select(dbmodels.NewWebhookFields...).
		From(dbmodels.TABLE_NEW_WEBHOOKS).
		Where(squirrel.Eq{"organization_id": orgId}).
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptNewWebhook)
}

func (repo *WebhookRepositoryPostgresql) ListWebhooksByEventType(ctx context.Context, exec Executor, orgId uuid.UUID, eventType string) ([]models.NewWebhook, error) {
	// PostgreSQL array contains operator
	query := NewQueryBuilder().
		Select(dbmodels.NewWebhookFields...).
		From(dbmodels.TABLE_NEW_WEBHOOKS).
		Where(squirrel.Eq{"organization_id": orgId}).
		Where(squirrel.Eq{"enabled": true}).
		Where(squirrel.Eq{"deleted_at": nil}).
		Where("? = ANY(event_types)", eventType)

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptNewWebhook)
}

func (repo *WebhookRepositoryPostgresql) UpdateWebhook(ctx context.Context, exec Executor, id uuid.UUID, update models.NewWebhookUpdate) error {
	builder := NewQueryBuilder().
		Update(dbmodels.TABLE_NEW_WEBHOOKS).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"deleted_at": nil})

	if update.EventTypes != nil {
		builder = builder.Set("event_types", *update.EventTypes)
	}
	if update.Url != nil {
		builder = builder.Set("url", *update.Url)
	}
	if update.HttpTimeoutSeconds != nil {
		builder = builder.Set("http_timeout_seconds", *update.HttpTimeoutSeconds)
	}
	if update.RateLimit != nil {
		builder = builder.Set("rate_limit", *update.RateLimit)
	}
	if update.RateLimitDurationSeconds != nil {
		builder = builder.Set("rate_limit_duration_seconds", *update.RateLimitDurationSeconds)
	}
	if update.Enabled != nil {
		builder = builder.Set("enabled", *update.Enabled)
	}

	return ExecBuilder(ctx, exec, builder)
}

func (repo *WebhookRepositoryPostgresql) DeleteWebhook(ctx context.Context, exec Executor, id uuid.UUID) error {
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_NEW_WEBHOOKS).
		Set("deleted_at", time.Now()).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"deleted_at": nil})

	return ExecBuilder(ctx, exec, query)
}

// Secret management

func (repo *WebhookRepositoryPostgresql) AddSecret(ctx context.Context, exec Executor, secret models.NewWebhookSecret) error {
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_WEBHOOK_SECRETS).
		Columns(
			"id",
			"webhook_id",
			"secret_value",
			"expires_at",
		).
		Values(
			secret.Id,
			secret.WebhookId,
			secret.Value,
			secret.ExpiresAt,
		)

	return ExecBuilder(ctx, exec, query)
}

func (repo *WebhookRepositoryPostgresql) ListActiveSecrets(ctx context.Context, exec Executor, webhookId uuid.UUID) ([]models.NewWebhookSecret, error) {
	query := NewQueryBuilder().
		Select(dbmodels.WebhookSecretFields...).
		From(dbmodels.TABLE_WEBHOOK_SECRETS).
		Where(squirrel.Eq{"webhook_id": webhookId}).
		Where(squirrel.Eq{"revoked_at": nil}).
		Where(squirrel.Or{
			squirrel.Eq{"expires_at": nil},
			squirrel.Gt{"expires_at": time.Now()},
		}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptNewWebhookSecret)
}

func (repo *WebhookRepositoryPostgresql) RevokeSecret(ctx context.Context, exec Executor, secretId uuid.UUID) error {
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOK_SECRETS).
		Set("revoked_at", time.Now()).
		Where(squirrel.Eq{"id": secretId}).
		Where(squirrel.Eq{"revoked_at": nil})

	return ExecBuilder(ctx, exec, query)
}

// Queue management

func (repo *WebhookRepositoryPostgresql) CreateWebhookQueueItem(ctx context.Context, exec Executor, item models.WebhookQueueItem) error {
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_WEBHOOK_QUEUE).
		Columns(
			"id",
			"organization_id",
			"event_type",
			"event_data",
		).
		Values(
			item.Id,
			item.OrganizationId,
			item.EventType,
			item.EventData,
		)

	return ExecBuilder(ctx, exec, query)
}

func (repo *WebhookRepositoryPostgresql) GetWebhookQueueItem(ctx context.Context, exec Executor, id uuid.UUID) (models.WebhookQueueItem, error) {
	query := NewQueryBuilder().
		Select(dbmodels.WebhookQueueItemFields...).
		From(dbmodels.TABLE_WEBHOOK_QUEUE).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptWebhookQueueItem)
}

// Delivery tracking

func (repo *WebhookRepositoryPostgresql) CreateDelivery(ctx context.Context, exec Executor, delivery models.WebhookDelivery) error {
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Columns(
			"id",
			"webhook_event_id",
			"webhook_id",
			"status",
			"attempts",
		).
		Values(
			delivery.Id,
			delivery.WebhookEventId,
			delivery.WebhookId,
			delivery.Status,
			delivery.Attempts,
		)

	return ExecBuilder(ctx, exec, query)
}

func (repo *WebhookRepositoryPostgresql) GetDelivery(ctx context.Context, exec Executor, id uuid.UUID) (models.WebhookDelivery, error) {
	query := NewQueryBuilder().
		Select(dbmodels.WebhookDeliveryFields...).
		From(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptWebhookDelivery)
}

func (repo *WebhookRepositoryPostgresql) DeliveryExists(ctx context.Context, exec Executor, webhookEventId, webhookId uuid.UUID) (bool, error) {
	query := NewQueryBuilder().
		Select("1").
		From(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Where(squirrel.Eq{"webhook_event_id": webhookEventId}).
		Where(squirrel.Eq{"webhook_id": webhookId}).
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return false, errors.Wrap(err, "error building query")
	}

	var exists int
	err = exec.QueryRow(ctx, sql, args...).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, "error checking delivery existence")
	}
	return true, nil
}

func (repo *WebhookRepositoryPostgresql) UpdateDeliverySuccess(ctx context.Context, exec Executor, id uuid.UUID, responseStatus int) error {
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Set("status", models.WebhookDeliveryStatusSuccess).
		Set("last_response_status", responseStatus).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id})

	return ExecBuilder(ctx, exec, query)
}

func (repo *WebhookRepositoryPostgresql) UpdateDeliveryFailed(ctx context.Context, exec Executor, id uuid.UUID, errMsg string, responseStatus *int) error {
	builder := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Set("status", models.WebhookDeliveryStatusFailed).
		Set("last_error", errMsg).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id})

	if responseStatus != nil {
		builder = builder.Set("last_response_status", *responseStatus)
	}

	return ExecBuilder(ctx, exec, builder)
}

func (repo *WebhookRepositoryPostgresql) UpdateDeliveryAttempt(ctx context.Context, exec Executor, id uuid.UUID, errMsg string, responseStatus *int, attempts int, nextRetryAt time.Time) error {
	builder := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Set("attempts", attempts).
		Set("last_error", errMsg).
		Set("next_retry_at", nextRetryAt).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id})

	if responseStatus != nil {
		builder = builder.Set("last_response_status", *responseStatus)
	}

	return ExecBuilder(ctx, exec, builder)
}
