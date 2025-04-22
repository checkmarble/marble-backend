package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
)

func (repo MarbleDbRepository) GetDataModelOptionsForTable(ctx context.Context, exec Executor, tableId string) (*models.DataModelOptions, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectDataModelOptionsColumns...).
		From(dbmodels.TABLE_DATA_MODEL_OPTIONS).
		Where(squirrel.Eq{"table_id": tableId}).
		Limit(1)

	return SqlToOptionalModel(ctx, exec, sql, dbmodels.AdaptDataModelOptions)
}

func (repo MarbleDbRepository) UpsertDataModelOptions(ctx context.Context, exec Executor,
	req models.UpdateDataModelOptionsRequest,
) (models.DataModelOptions, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.DataModelOptions{}, err
	}

	updateFields := make([]string, 0)

	if req.DisplayedFields != nil {
		updateFields = append(updateFields, "displayed_fields = excluded.displayed_fields")
	}
	if req.FieldOrder != nil {
		updateFields = append(updateFields, "field_order = excluded.field_order")
	}

	if len(updateFields) == 0 {
		return models.DataModelOptions{}, errors.New("nothing to update")
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_DATA_MODEL_OPTIONS).
		Columns("table_id", "displayed_fields", "field_order").
		Values(req.TableId, req.DisplayedFields, req.FieldOrder).
		Suffix(fmt.Sprintf("on conflict (table_id) do update set %s", strings.Join(updateFields, ","))).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptDataModelOptions)
}
