package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
)

func (repo MarbleDbRepository) GetAuditEvent(ctx context.Context, exec Executor, id string) (models.AuditEvent, error) {
	query := NewQueryBuilder().
		Select(dbmodels.SelectAuditEventColumns...).
		From(dbmodels.TABLE_AUDIT_EVENTS).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptAuditEvent)
}

func (repo MarbleDbRepository) buildAuditEventsQuery(filters dto.AuditEventFilters, limit int) (squirrel.SelectBuilder, error) {
	query := NewQueryBuilder().
		Select(append(
			columnsNames("ae", dbmodels.SelectAuditEventColumns),
			"u.first_name || ' ' || u.last_name as user_name",
			"ak.prefix as api_key_name",
		)...).
		From(dbmodels.TABLE_AUDIT_EVENTS+" ae").
		LeftJoin(dbmodels.TABLE_USERS+" u on u.id = ae.user_id::uuid").
		LeftJoin(dbmodels.TABLE_APIKEYS+" ak on ak.id = ae.api_key_id").
		Where("ae.org_id = ?", filters.OrgId).
		OrderBy("ae.created_at desc, id desc").
		Limit(uint64(limit))

	if filters.From != nil {
		query = query.Where("ae.created_at >= ?", *filters.From)
	}
	if filters.To != nil {
		query = query.Where("ae.created_at <= ?", *filters.To)
	}

	if filters.UserId != "" {
		query = query.Where("ae.user_id = ?", filters.UserId)
	}
	if filters.ApiKeyId != "" {
		query = query.Where("ae.api_key_id = ?", filters.ApiKeyId)
	}
	if filters.Table != "" {
		query = query.Where("ae.table = ?", filters.Table)
	}
	if filters.EntityId != "" {
		query = query.Where("ae.entity_id = ?", filters.EntityId)
	}

	return query, nil
}

func (repo MarbleDbRepository) ListAuditEvents(ctx context.Context, exec Executor, pagination models.PaginationAndSorting, filters dto.AuditEventFilters) ([]models.AuditEvent, error) {
	filters.Limit = pagination.Limit

	query, err := repo.buildAuditEventsQuery(filters, pagination.Limit)
	if err != nil {
		return nil, err
	}

	if pagination.OffsetId != "" {
		cursor, err := repo.GetAuditEvent(ctx, exec, pagination.OffsetId)
		if err != nil {
			return nil, errors.Wrap(err, "could not retrieve cursor event")
		}

		query = query.Where("(ae.created_at, ae.id) < (?, ?)", cursor.CreatedAt, cursor.Id)
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptAuditEventWithActor)
}

func (repo MarbleDbRepository) DownloadAuditEvents(ctx context.Context, exec Executor, filters dto.AuditEventFilters) (models.ChannelOfModels[models.AuditEvent], error) {
	query, err := repo.buildAuditEventsQuery(filters, filters.Limit)
	if err != nil {
		return models.ChannelOfModels[models.AuditEvent]{}, err
	}

	if filters.After != "" {
		cursor, err := repo.GetAuditEvent(ctx, exec, filters.After)
		if err != nil {
			return models.ChannelOfModels[models.AuditEvent]{}, errors.Wrap(err, "could not retrieve cursor event")
		}

		query = query.Where("(ae.created_at, ae.id) < (?, ?)", cursor.CreatedAt, cursor.Id)
	}

	return SqlToChannelOfModel(ctx, exec, query, func(row pgx.CollectableRow) (models.AuditEvent, error) {
		model, err := pgx.RowToStructByName[dbmodels.DbAuditEventWithActor](row)
		if err != nil {
			return models.AuditEvent{}, err
		}
		return dbmodels.AdaptAuditEventWithActor(model)
	}), nil
}
