package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) AddScreeningMatchWhitelist(ctx context.Context, exec Executor,
	orgId, counterpartyId string, entityId string, reviewerId *models.UserId,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCREENING_WHITELISTS).
		Columns("org_id", "counterparty_id", "entity_id", "whitelisted_by").
		Values(orgId, counterpartyId, entityId, reviewerId).
		Suffix("ON CONFLICT (org_id, counterparty_id, entity_id) DO NOTHING")

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) DeleteScreeningMatchWhitelist(ctx context.Context, exec Executor,
	orgId string, counterpartyId *string, entityId string, reviewerId *models.UserId,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	filters := squirrel.Eq{
		"org_id":    orgId,
		"entity_id": entityId,
	}

	if counterpartyId != nil {
		filters["counterparty_id"] = counterpartyId
	}

	sql := NewQueryBuilder().
		Delete(dbmodels.TABLE_SCREENING_WHITELISTS).
		Where(filters)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) SearchScreeningMatchWhitelist(ctx context.Context,
	exec Executor, orgId string, counterpartyId, entityId *string,
) ([]models.ScreeningWhitelist, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	filters := squirrel.Eq{"org_id": orgId}

	if entityId != nil {
		filters["entity_id"] = entityId
	}
	if counterpartyId != nil {
		filters["counterparty_id"] = counterpartyId
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ScreeningWhitelistColumnList...).
		From(dbmodels.TABLE_SCREENING_WHITELISTS).
		Where(filters)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreeningWhitelist)
}

func (repo *MarbleDbRepository) SearchScreeningMatchWhitelistByIds(
	ctx context.Context,
	exec Executor,
	orgId string,
	counterpartyIds, entityIds []string,
) ([]models.ScreeningWhitelist, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	filters := squirrel.Eq{"org_id": orgId}

	if len(entityIds) > 0 {
		filters["entity_id"] = entityIds
	}
	if len(counterpartyIds) > 0 {
		filters["counterparty_id"] = counterpartyIds
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ScreeningWhitelistColumnList...).
		From(dbmodels.TABLE_SCREENING_WHITELISTS).
		Where(filters)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreeningWhitelist)
}

func (repo *MarbleDbRepository) IsScreeningMatchWhitelisted(ctx context.Context, exec Executor,
	orgId, counterpartyId string, entityIds []string,
) ([]models.ScreeningWhitelist, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ScreeningWhitelistColumnList...).
		From(dbmodels.TABLE_SCREENING_WHITELISTS).
		Where(squirrel.And{
			squirrel.Eq{
				"org_id":          orgId,
				"counterparty_id": counterpartyId,
			},
			squirrel.Expr("entity_id = ANY(?)", entityIds),
		})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreeningWhitelist)
}
