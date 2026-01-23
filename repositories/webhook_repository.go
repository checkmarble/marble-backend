package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func selectWebhooks() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.WebhookFields...).
		From(dbmodels.TABLE_WEBHOOKS)
}

func selectWebhookSecrets() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.WebhookSecretFields...).
		From(dbmodels.TABLE_WEBHOOK_SECRETS)
}

func (repo MarbleDbRepository) CreateWebhook(
	ctx context.Context,
	exec Executor,
	input models.WebhookRegister,
	orgId uuid.UUID,
	partnerId *uuid.UUID,
	secretValue string,
) (models.Webhook, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Webhook{}, err
	}

	webhookId := uuid.New()
	secretId := uuid.New()
	now := time.Now()

	// Insert webhook
	webhookInsert := NewQueryBuilder().
		Insert(dbmodels.TABLE_WEBHOOKS).
		Columns(
			"id",
			"organization_id",
			"partner_id",
			"url",
			"event_types",
			"http_timeout_seconds",
			"rate_limit",
			"rate_limit_duration_seconds",
			"enabled",
			"created_at",
			"updated_at",
		).
		Values(
			webhookId,
			orgId,
			partnerId,
			input.Url,
			input.EventTypes,
			input.HttpTimeout,
			input.RateLimit,
			input.RateLimitDuration,
			true,
			now,
			now,
		)

	if err := ExecBuilder(ctx, exec, webhookInsert); err != nil {
		return models.Webhook{}, err
	}

	// Insert secret
	secretInsert := NewQueryBuilder().
		Insert(dbmodels.TABLE_WEBHOOK_SECRETS).
		Columns(
			"id",
			"webhook_id",
			"secret_value",
			"created_at",
		).
		Values(
			secretId,
			webhookId,
			secretValue,
			now,
		)

	if err := ExecBuilder(ctx, exec, secretInsert); err != nil {
		return models.Webhook{}, err
	}

	webhook := models.Webhook{
		Id:                webhookId,
		OrganizationId:    orgId,
		EventTypes:        input.EventTypes,
		Url:               input.Url,
		HttpTimeout:       input.HttpTimeout,
		RateLimit:         input.RateLimit,
		RateLimitDuration: input.RateLimitDuration,
		Enabled:           true,
		CreatedAt:         now,
		UpdatedAt:         now,
		Secrets: []models.Secret{{
			Id:        secretId,
			WebhookId: webhookId,
			Value:     secretValue,
			CreatedAt: now,
		}},
	}

	if partnerId != nil {
		webhook.PartnerId = null.StringFrom(partnerId.String())
	}

	return webhook, nil
}

func (repo MarbleDbRepository) GetWebhook(ctx context.Context, exec Executor, id uuid.UUID) (models.Webhook, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Webhook{}, err
	}

	webhook, err := SqlToModel(
		ctx,
		exec,
		selectWebhooks().Where(squirrel.Eq{"id": id}).Where(squirrel.Eq{"deleted_at": nil}),
		dbmodels.AdaptWebhook,
	)
	if err != nil {
		return models.Webhook{}, err
	}

	// Get secrets
	secrets, err := repo.ListActiveSecrets(ctx, exec, id)
	if err != nil {
		return models.Webhook{}, err
	}
	webhook.Secrets = secrets

	return webhook, nil
}

func (repo MarbleDbRepository) ListWebhooks(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	partnerId *uuid.UUID,
) ([]models.Webhook, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectWebhooks().
		Where(squirrel.Eq{"organization_id": orgId}).
		Where(squirrel.Eq{"deleted_at": nil})

	if partnerId != nil {
		query = query.Where(squirrel.Eq{"partner_id": partnerId})
	}

	webhooks, err := SqlToListOfRow(
		ctx,
		exec,
		query,
		func(row pgx.CollectableRow) (models.Webhook, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DBWebhook](row)
			if err != nil {
				return models.Webhook{}, err
			}
			return dbmodels.AdaptWebhook(db)
		},
	)
	if err != nil {
		return nil, err
	}

	// Get secrets for each webhook
	for i := range webhooks {
		secrets, err := repo.ListActiveSecrets(ctx, exec, webhooks[i].Id)
		if err != nil {
			return nil, err
		}
		webhooks[i].Secrets = secrets
	}

	return webhooks, nil
}

func (repo MarbleDbRepository) ListWebhooksByEventType(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	partnerId *uuid.UUID,
	eventType string,
) ([]models.Webhook, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	// Query webhooks where event_types array contains the eventType or is empty (meaning all events)
	query := selectWebhooks().
		Where(squirrel.Eq{"organization_id": orgId}).
		Where(squirrel.Eq{"deleted_at": nil}).
		Where(squirrel.Eq{"enabled": true}).
		Where(squirrel.Or{
			squirrel.Expr("? = ANY(event_types)", eventType),
			squirrel.Expr("array_length(event_types, 1) IS NULL"),
			squirrel.Expr("event_types = '{}'"),
		})

	if partnerId != nil {
		query = query.Where(squirrel.Eq{"partner_id": partnerId})
	}

	webhooks, err := SqlToListOfRow(
		ctx,
		exec,
		query,
		func(row pgx.CollectableRow) (models.Webhook, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DBWebhook](row)
			if err != nil {
				return models.Webhook{}, err
			}
			return dbmodels.AdaptWebhook(db)
		},
	)
	if err != nil {
		return nil, err
	}

	// Get secrets for each webhook
	for i := range webhooks {
		secrets, err := repo.ListActiveSecrets(ctx, exec, webhooks[i].Id)
		if err != nil {
			return nil, err
		}
		webhooks[i].Secrets = secrets
	}

	return webhooks, nil
}

func (repo MarbleDbRepository) UpdateWebhook(
	ctx context.Context,
	exec Executor,
	webhook models.Webhook,
) (models.Webhook, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Webhook{}, err
	}

	now := time.Now()

	update := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOKS).
		Set("url", webhook.Url).
		Set("event_types", webhook.EventTypes).
		Set("http_timeout_seconds", webhook.HttpTimeout).
		Set("rate_limit", webhook.RateLimit).
		Set("rate_limit_duration_seconds", webhook.RateLimitDuration).
		Set("enabled", webhook.Enabled).
		Set("updated_at", now).
		Where(squirrel.Eq{"id": webhook.Id}).
		Where(squirrel.Eq{"deleted_at": nil})

	if err := ExecBuilder(ctx, exec, update); err != nil {
		return models.Webhook{}, err
	}

	// Return the updated webhook
	return repo.GetWebhook(ctx, exec, webhook.Id)
}

func (repo MarbleDbRepository) DeleteWebhook(ctx context.Context, exec Executor, id uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	// Soft delete
	update := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOKS).
		Set("deleted_at", time.Now()).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id})

	return ExecBuilder(ctx, exec, update)
}

// Secret management

func (repo MarbleDbRepository) AddWebhookSecret(
	ctx context.Context,
	exec Executor,
	webhookId uuid.UUID,
	secretValue string,
) (models.Secret, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Secret{}, err
	}

	secretId := uuid.New()
	now := time.Now()

	insert := NewQueryBuilder().
		Insert(dbmodels.TABLE_WEBHOOK_SECRETS).
		Columns(
			"id",
			"webhook_id",
			"secret_value",
			"created_at",
		).
		Values(
			secretId,
			webhookId,
			secretValue,
			now,
		)

	if err := ExecBuilder(ctx, exec, insert); err != nil {
		return models.Secret{}, err
	}

	return models.Secret{
		Id:        secretId,
		WebhookId: webhookId,
		Value:     secretValue,
		CreatedAt: now,
	}, nil
}

func (repo MarbleDbRepository) RevokeWebhookSecret(ctx context.Context, exec Executor, secretId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	update := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOK_SECRETS).
		Set("revoked_at", time.Now()).
		Where(squirrel.Eq{"id": secretId})

	return ExecBuilder(ctx, exec, update)
}

func (repo MarbleDbRepository) ListActiveSecrets(
	ctx context.Context,
	exec Executor,
	webhookId uuid.UUID,
) ([]models.Secret, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectWebhookSecrets().
		Where(squirrel.Eq{"webhook_id": webhookId}).
		Where(squirrel.Eq{"revoked_at": nil}).
		Where(squirrel.Or{
			squirrel.Eq{"expires_at": nil},
			squirrel.Expr("expires_at > NOW()"),
		}).
		OrderBy("created_at DESC")

	return SqlToListOfRow(
		ctx,
		exec,
		query,
		func(row pgx.CollectableRow) (models.Secret, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DBWebhookSecret](row)
			if err != nil {
				return models.Secret{}, err
			}
			return dbmodels.AdaptWebhookSecret(db), nil
		},
	)
}

// Webhook Deliveries

func selectWebhookDeliveries() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.WebhookDeliveryFields...).
		From(dbmodels.TABLE_WEBHOOK_DELIVERIES)
}

func (repo MarbleDbRepository) CreateWebhookDeliveries(
	ctx context.Context,
	exec Executor,
	eventId uuid.UUID,
	webhookIds []uuid.UUID,
) ([]models.WebhookDelivery, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if len(webhookIds) == 0 {
		return []models.WebhookDelivery{}, nil
	}

	now := time.Now()
	deliveries := make([]models.WebhookDelivery, 0, len(webhookIds))

	insert := NewQueryBuilder().
		Insert(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Columns(
			"id",
			"webhook_event_id",
			"webhook_id",
			"status",
			"attempts",
			"next_retry_at",
			"created_at",
			"updated_at",
		)

	for _, webhookId := range webhookIds {
		deliveryId := uuid.New()
		insert = insert.Values(
			deliveryId,
			eventId,
			webhookId,
			models.DeliveryPending,
			0,
			now,
			now,
			now,
		)
		deliveries = append(deliveries, models.WebhookDelivery{
			Id:             deliveryId,
			WebhookEventId: eventId,
			WebhookId:      webhookId,
			Status:         models.DeliveryPending,
			Attempts:       0,
			NextRetryAt:    &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}

	if err := ExecBuilder(ctx, exec, insert); err != nil {
		return nil, err
	}

	return deliveries, nil
}

func (repo MarbleDbRepository) GetWebhookDelivery(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
) (models.WebhookDelivery, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WebhookDelivery{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectWebhookDeliveries().Where(squirrel.Eq{"id": id}),
		dbmodels.AdaptWebhookDelivery,
	)
}

func (repo MarbleDbRepository) ListPendingWebhookDeliveries(
	ctx context.Context,
	exec Executor,
	limit int,
) ([]models.WebhookDelivery, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectWebhookDeliveries().
		Where(squirrel.Eq{"status": models.DeliveryPending}).
		Where(squirrel.LtOrEq{"next_retry_at": time.Now()}).
		OrderBy("next_retry_at ASC").
		Limit(uint64(limit))

	return SqlToListOfRow(
		ctx,
		exec,
		query,
		func(row pgx.CollectableRow) (models.WebhookDelivery, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DBWebhookDelivery](row)
			if err != nil {
				return models.WebhookDelivery{}, err
			}
			return dbmodels.AdaptWebhookDelivery(db)
		},
	)
}

func (repo MarbleDbRepository) ListPendingWebhookDeliveriesForOrg(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	limit int,
) ([]models.WebhookDelivery, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	// Join with webhooks to filter by organization
	query := NewQueryBuilder().
		Select(
			"d.id",
			"d.webhook_event_id",
			"d.webhook_id",
			"d.status",
			"d.attempts",
			"d.next_retry_at",
			"d.last_error",
			"d.last_response_status",
			"d.created_at",
			"d.updated_at",
		).
		From(dbmodels.TABLE_WEBHOOK_DELIVERIES + " d").
		Join(dbmodels.TABLE_WEBHOOKS + " w ON d.webhook_id = w.id").
		Where(squirrel.Eq{"d.status": models.DeliveryPending}).
		Where(squirrel.LtOrEq{"d.next_retry_at": time.Now()}).
		Where(squirrel.Eq{"w.organization_id": orgId}).
		OrderBy("d.next_retry_at ASC").
		Limit(uint64(limit))

	return SqlToListOfRow(
		ctx,
		exec,
		query,
		func(row pgx.CollectableRow) (models.WebhookDelivery, error) {
			var db dbmodels.DBWebhookDelivery
			err := row.Scan(
				&db.Id,
				&db.WebhookEventId,
				&db.WebhookId,
				&db.Status,
				&db.Attempts,
				&db.NextRetryAt,
				&db.LastError,
				&db.LastResponseStatus,
				&db.CreatedAt,
				&db.UpdatedAt,
			)
			if err != nil {
				return models.WebhookDelivery{}, err
			}
			return dbmodels.AdaptWebhookDelivery(db)
		},
	)
}

func (repo MarbleDbRepository) ListWebhookDeliveriesForEvent(
	ctx context.Context,
	exec Executor,
	eventId uuid.UUID,
) ([]models.WebhookDelivery, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectWebhookDeliveries().
		Where(squirrel.Eq{"webhook_event_id": eventId}).
		OrderBy("created_at ASC")

	return SqlToListOfRow(
		ctx,
		exec,
		query,
		func(row pgx.CollectableRow) (models.WebhookDelivery, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DBWebhookDelivery](row)
			if err != nil {
				return models.WebhookDelivery{}, err
			}
			return dbmodels.AdaptWebhookDelivery(db)
		},
	)
}

func (repo MarbleDbRepository) UpdateWebhookDelivery(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	update models.WebhookDeliveryUpdate,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	var nextRetryAt pgtype.Timestamptz
	if update.NextRetryAt != nil {
		nextRetryAt = pgtype.Timestamptz{Time: *update.NextRetryAt, Valid: true}
	}

	var lastError pgtype.Text
	if update.LastError != nil {
		lastError = pgtype.Text{String: *update.LastError, Valid: true}
	}

	var lastResponseStatus pgtype.Int4
	if update.LastResponseStatus != nil {
		lastResponseStatus = pgtype.Int4{Int32: int32(*update.LastResponseStatus), Valid: true}
	}

	updateQuery := NewQueryBuilder().
		Update(dbmodels.TABLE_WEBHOOK_DELIVERIES).
		Set("status", update.Status).
		Set("attempts", update.Attempts).
		Set("next_retry_at", nextRetryAt).
		Set("last_error", lastError).
		Set("last_response_status", lastResponseStatus).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id})

	return ExecBuilder(ctx, exec, updateQuery)
}

func (repo MarbleDbRepository) MarkWebhookDeliverySuccess(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	responseStatus int,
) error {
	return repo.UpdateWebhookDelivery(ctx, exec, id, models.WebhookDeliveryUpdate{
		Status:             models.DeliverySuccess,
		LastResponseStatus: &responseStatus,
	})
}

func (repo MarbleDbRepository) MarkWebhookDeliveryFailed(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	errMsg string,
	responseStatus *int,
	nextRetryAt *time.Time,
	attempts int,
) error {
	status := models.DeliveryPending
	if nextRetryAt == nil {
		status = models.DeliveryFailed
	}

	return repo.UpdateWebhookDelivery(ctx, exec, id, models.WebhookDeliveryUpdate{
		Status:             status,
		Attempts:           attempts,
		NextRetryAt:        nextRetryAt,
		LastError:          &errMsg,
		LastResponseStatus: responseStatus,
	})
}
