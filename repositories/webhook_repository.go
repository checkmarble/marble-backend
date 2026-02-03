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

// Webhook CRUD

func (repo *MarbleDbRepository) CreateWebhook(ctx context.Context, exec Executor, webhook models.NewWebhook) error {
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

func (repo *MarbleDbRepository) GetWebhook(ctx context.Context, exec Executor, id uuid.UUID) (models.NewWebhook, error) {
	query := NewQueryBuilder().
		Select(dbmodels.NewWebhookFields...).
		From(dbmodels.TABLE_NEW_WEBHOOKS).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"deleted_at": nil})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptNewWebhook)
}

func (repo *MarbleDbRepository) ListWebhooks(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.NewWebhook, error) {
	query := NewQueryBuilder().
		Select(dbmodels.NewWebhookFields...).
		From(dbmodels.TABLE_NEW_WEBHOOKS).
		Where(squirrel.Eq{"organization_id": orgId}).
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptNewWebhook)
}

func (repo *MarbleDbRepository) ListWebhooksByEventType(ctx context.Context, exec Executor, orgId uuid.UUID, eventType string) ([]models.NewWebhook, error) {
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

func (repo *MarbleDbRepository) UpdateWebhook(ctx context.Context, exec Executor, id uuid.UUID, update models.NewWebhookUpdate) error {
	// Early exit if nothing to update
	if update.EventTypes == nil && update.Url == nil && update.HttpTimeoutSeconds == nil &&
		update.RateLimit == nil && update.RateLimitDurationSeconds == nil && update.Enabled == nil {
		return nil
	}

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

func (repo *MarbleDbRepository) DeleteWebhook(ctx context.Context, exec Executor, id uuid.UUID) error {
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_NEW_WEBHOOKS).
		Set("deleted_at", time.Now()).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"deleted_at": nil})

	return ExecBuilder(ctx, exec, query)
}

// Secret management

func (repo *MarbleDbRepository) AddWebhookSecret(ctx context.Context, exec Executor, secret models.NewWebhookSecret) error {
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

func (repo *MarbleDbRepository) ListActiveWebhookSecrets(ctx context.Context, exec Executor, webhookId uuid.UUID) ([]models.NewWebhookSecret, error) {
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

func (repo *MarbleDbRepository) RevokeWebhookSecret(ctx context.Context, exec Executor, secretId uuid.UUID) error {
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOK_SECRETS).
		Set("revoked_at", time.Now()).
		Where(squirrel.Eq{"id": secretId}).
		Where(squirrel.Eq{"revoked_at": nil})

	return ExecBuilder(ctx, exec, query)
}

// Webhook events (v2)

func (repo *MarbleDbRepository) CreateWebhookEventV2(ctx context.Context, exec Executor, event models.WebhookEventV2) error {
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_WEBHOOK_EVENTS_V2).
		Columns(
			"id",
			"organization_id",
			"event_type",
			"event_data",
		).
		Values(
			event.Id,
			event.OrganizationId,
			event.EventType,
			event.EventData,
		)

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) GetWebhookEventV2(ctx context.Context, exec Executor, id uuid.UUID) (models.WebhookEventV2, error) {
	query := NewQueryBuilder().
		Select(dbmodels.WebhookEventV2Fields...).
		From(dbmodels.TABLE_WEBHOOK_EVENTS_V2).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptWebhookEventV2)
}

func (repo *MarbleDbRepository) DeleteWebhookEventV2(ctx context.Context, exec Executor, id uuid.UUID) error {
	query := NewQueryBuilder().
		Delete(dbmodels.TABLE_WEBHOOK_EVENTS_V2).
		Where(squirrel.Eq{"id": id})

	return ExecBuilder(ctx, exec, query)
}

// Delivery tracking

func (repo *MarbleDbRepository) CreateWebhookDelivery(ctx context.Context, exec Executor, delivery models.WebhookDelivery) error {
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

func (repo *MarbleDbRepository) GetWebhookDelivery(ctx context.Context, exec Executor, id uuid.UUID) (models.WebhookDelivery, error) {
	query := NewQueryBuilder().
		Select(dbmodels.WebhookDeliveryFields...).
		From(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptWebhookDelivery)
}

func (repo *MarbleDbRepository) WebhookDeliveryExists(ctx context.Context, exec Executor, webhookEventId, webhookId uuid.UUID) (bool, error) {
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

func (repo *MarbleDbRepository) UpdateWebhookDeliverySuccess(ctx context.Context, exec Executor, id uuid.UUID, responseStatus int) error {
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Set("status", models.WebhookDeliveryStatusSuccess).
		Set("last_response_status", responseStatus).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id})

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) UpdateWebhookDeliveryFailed(ctx context.Context, exec Executor, id uuid.UUID, errMsg string, responseStatus *int) error {
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

func (repo *MarbleDbRepository) UpdateWebhookDeliveryAttempt(ctx context.Context, exec Executor, id uuid.UUID, errMsg string, responseStatus *int, attempts int, nextRetryAt time.Time) error {
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

// CountPendingWebhookDeliveries returns the number of pending deliveries for an event
func (repo *MarbleDbRepository) CountPendingWebhookDeliveries(ctx context.Context, exec Executor, webhookEventId uuid.UUID) (int, error) {
	query := NewQueryBuilder().
		Select("COUNT(*)").
		From(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Where(squirrel.Eq{"webhook_event_id": webhookEventId}).
		Where(squirrel.Eq{"status": models.WebhookDeliveryStatusPending})

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "error building query")
	}

	var count int
	err = exec.QueryRow(ctx, sql, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "error counting pending deliveries")
	}
	return count, nil
}

// Cleanup methods

// DeleteOldWebhookDeliveries deletes deliveries older than the specified retention period
// that are in a terminal state (success or failed)
func (repo *MarbleDbRepository) DeleteOldWebhookDeliveries(ctx context.Context, exec Executor, olderThan time.Time) (int64, error) {
	query := NewQueryBuilder().
		Delete(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Where(squirrel.Lt{"updated_at": olderThan}).
		Where(squirrel.Or{
			squirrel.Eq{"status": models.WebhookDeliveryStatusSuccess},
			squirrel.Eq{"status": models.WebhookDeliveryStatusFailed},
		})

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "error building query")
	}

	result, err := exec.Exec(ctx, sql, args...)
	if err != nil {
		return 0, errors.Wrap(err, "error deleting old deliveries")
	}
	return result.RowsAffected(), nil
}

// DeleteOrphanedWebhookEventsV2 deletes events that have no associated deliveries
func (repo *MarbleDbRepository) DeleteOrphanedWebhookEventsV2(ctx context.Context, exec Executor) (int64, error) {
	sql := `
		DELETE FROM webhook_events_v2 e
		WHERE NOT EXISTS (
			SELECT 1 FROM webhook_deliveries d WHERE d.webhook_event_id = e.id
		)
	`

	result, err := exec.Exec(ctx, sql)
	if err != nil {
		return 0, errors.Wrap(err, "error deleting orphaned events")
	}
	return result.RowsAffected(), nil
}
