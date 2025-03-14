package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) AddSanctionCheckMatchWhitelist(ctx context.Context, exec Executor,
	orgId, counterpartyId string, entityId string, reviewerId *models.UserId,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_WHITELISTS).
		Columns("org_id", "counterparty_id", "entity_id", "whitelisted_by").
		Values(orgId, counterpartyId, entityId, reviewerId).
		Suffix("ON CONFLICT (org_id, counterparty_id, entity_id) DO NOTHING")

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) DeleteSanctionCheckMatchWhitelist(ctx context.Context, exec Executor,
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
		Delete(dbmodels.TABLE_SANCTION_CHECK_WHITELISTS).
		Where(filters)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) SearchSanctionCheckMatchWhitelist(ctx context.Context,
	exec Executor, orgId string, counterpartyId, entityId *string,
) ([]models.SanctionCheckWhitelist, error) {
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
		Select(dbmodels.SanctionCheckWhitelistColumnList...).
		From(dbmodels.TABLE_SANCTION_CHECK_WHITELISTS).
		Where(filters)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckWhitelist)
}

func (repo *MarbleDbRepository) IsSanctionCheckMatchWhitelisted(ctx context.Context, exec Executor,
	orgId, counterpartyId string, entityIds []string,
) ([]models.SanctionCheckWhitelist, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SanctionCheckWhitelistColumnList...).
		From(dbmodels.TABLE_SANCTION_CHECK_WHITELISTS).
		Where(squirrel.And{
			squirrel.Eq{
				"org_id":          orgId,
				"counterparty_id": counterpartyId,
			},
			squirrel.Expr("entity_id = ANY(?)", entityIds),
		})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckWhitelist)
}
