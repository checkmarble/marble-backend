package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo MarbleDbRepository) ListCustomerAggregates(ctx context.Context, exec Executor, orgId, tableId uuid.UUID, limit int) ([]models.CustomerAggregate, error) {
	query := NewQueryBuilder().
		Select(dbmodels.SelectCustomerAggregatesColumns...).
		From(dbmodels.TABLE_CUSTOMER_AGGREGATES).
		Where("org_id = ?", orgId).
		Where("table_id = ?", tableId).
		Limit(uint64(limit))

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCustomerAggregate)
}

func (repo MarbleDbRepository) GetCustomerAggregate(ctx context.Context, exec Executor, orgId, id uuid.UUID) (models.CustomerAggregate, error) {
	query := NewQueryBuilder().
		Select(dbmodels.SelectCustomerAggregatesColumns...).
		From(dbmodels.TABLE_CUSTOMER_AGGREGATES).
		Where("org_id = ?", orgId).
		Where("id = ?", id)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptCustomerAggregate)
}

func (repo MarbleDbRepository) CreateCustomerAggregate(ctx context.Context, exec Executor, orgId, tableId uuid.UUID, req models.CreateCustomerAggregate) (models.CustomerAggregate, error) {
	expr, err := dbmodels.SerializeFormulaAstExpression(&req.Expression)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_CUSTOMER_AGGREGATES).
		Columns("id", "org_id", "table_id", "name", "kind", "expression", "time_slice").
		Values(
			pure_utils.NewId(),
			orgId,
			tableId,
			req.Name,
			string(req.Type),
			expr,
			req.TimeSlice,
		).
		Suffix(fmt.Sprintf("returning %s", strings.Join(dbmodels.SelectCustomerAggregatesColumns, ",")))

	return SqlToModel(ctx, exec, query, dbmodels.AdaptCustomerAggregate)
}

func (repo MarbleDbRepository) UpdateCustomerAggregate(ctx context.Context, exec Executor, orgId uuid.UUID, req models.UpdateCustomerAggregate) (models.CustomerAggregate, error) {
	expr, err := dbmodels.SerializeFormulaAstExpression(&req.Expression)
	if err != nil {
		return models.CustomerAggregate{}, err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CUSTOMER_AGGREGATES).
		Set("name", req.Name).
		Set("kind", string(req.Type)).
		Set("expression", expr).
		Set("time_slice", req.TimeSlice).
		Where("org_id = ?", orgId).
		Where("id = ?", req.Id).
		Suffix(fmt.Sprintf("returning %s", strings.Join(dbmodels.SelectCustomerAggregatesColumns, ",")))

	return SqlToModel(ctx, exec, query, dbmodels.AdaptCustomerAggregate)
}

func (repo MarbleDbRepository) DeleteCustomerAggregate(ctx context.Context, exec Executor, orgId, id uuid.UUID) error {
	query := NewQueryBuilder().
		Delete(dbmodels.TABLE_CUSTOMER_AGGREGATES).
		Where("org_id = ?", orgId).
		Where("id = ?", id)

	return ExecBuilder(ctx, exec, query)
}
