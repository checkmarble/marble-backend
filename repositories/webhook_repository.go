package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func selectWebhooks() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.WebhookFields...).
		From(dbmodels.TABLE_WEBHOOKS)
}

func (repo MarbleDbRepository) GetWebhook(ctx context.Context, exec Executor, webhookId string) (models.Webhook, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Webhook{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectWebhooks().Where(squirrel.Eq{"id": webhookId}),
		dbmodels.AdaptWebhook,
	)
}

func (repo MarbleDbRepository) ListWebhooks(ctx context.Context, exec Executor, filters models.WebhookFilters) ([]models.Webhook, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectWebhooks()
	mergedFilters := filters.MergeWithDefaults()

	if mergedFilters.DeliveryStatus != nil {
		query = query.Where(squirrel.Eq{"delivery_status": mergedFilters.DeliveryStatus})
	}

	query = query.OrderBy("created_at DESC").Limit(mergedFilters.Limit)

	return SqlToListOfRow(
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
}

func (repo MarbleDbRepository) CreateWebhook(
	ctx context.Context,
	exec Executor,
	webhookId string,
	webhook models.WebhookCreate,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_WEBHOOKS).
			Columns(
				"id",
				"delivery_status",
				"organization_id",
				"partner_id",
				"event_type",
				"event_data",
			).
			Values(
				webhookId,
				models.Scheduled.String(),
				webhook.OrganizationId,
				webhook.PartnerId.Ptr(),
				webhook.EventType.String(),
				webhook.EventData,
			),
	)
	return err
}

func (repo MarbleDbRepository) UpdateWebhook(
	ctx context.Context,
	exec Executor,
	input models.WebhookUpdate,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TABLE_WEBHOOKS).
			Set("updated_at", "NOW()").
			Set("delivery_status", input.DeliveryStatus.String()).
			Set("send_attempt_count", input.SendAttemptCount).
			Where(squirrel.Eq{"id": input.Id}),
	)
	return err
}
