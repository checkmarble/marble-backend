package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func selectWebhookEvents() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.WebhookEventFields...).
		From(dbmodels.TABLE_WEBHOOK_EVENTS)
}

func (repo MarbleDbRepository) GetWebhookEvent(ctx context.Context, exec Executor, webhookEventId string) (models.WebhookEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.WebhookEvent{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectWebhookEvents().Where(squirrel.Eq{"id": webhookEventId}),
		dbmodels.AdaptWebhookEvent,
	)
}

func (repo MarbleDbRepository) ListWebhookEvents(ctx context.Context, exec Executor,
	filters models.WebhookEventFilters,
) ([]models.WebhookEvent, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	mergedFilters := filters.MergeWithDefaults()

	query := selectWebhookEvents().Limit(mergedFilters.Limit)

	if mergedFilters.DeliveryStatus != nil {
		query = query.Where(squirrel.Eq{"delivery_status": mergedFilters.DeliveryStatus})
	}

	return SqlToListOfRow(
		ctx,
		exec,
		query,
		func(row pgx.CollectableRow) (models.WebhookEvent, error) {
			db, err := pgx.RowToStructByPos[dbmodels.DBWebhookEvent](row)
			if err != nil {
				return models.WebhookEvent{}, err
			}

			return dbmodels.AdaptWebhookEvent(db)
		},
	)
}

func (repo MarbleDbRepository) CreateWebhookEvent(
	ctx context.Context,
	exec Executor,
	webhookEventId string,
	input models.WebhookEventCreate,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_WEBHOOK_EVENTS).
			Columns(
				"id",
				"delivery_status",
				"organization_id",
				"partner_id",
				"event_type",
				"event_data",
			).
			Values(
				webhookEventId,
				models.Scheduled,
				input.OrganizationId,
				input.PartnerId.Ptr(),
				input.EventContent.Type,
				input.EventContent.Data,
			),
	)
	return err
}

func (repo MarbleDbRepository) UpdateWebhookEvent(
	ctx context.Context,
	exec Executor,
	input models.WebhookEventUpdate,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TABLE_WEBHOOK_EVENTS).
			Set("updated_at", "NOW()").
			Set("delivery_status", input.DeliveryStatus).
			Set("send_attempt_count", input.SendAttemptCount).
			Where(squirrel.Eq{"id": input.Id}),
	)
	return err
}
