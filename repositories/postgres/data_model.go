package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Masterminds/squirrel"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (db *Database) GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error) {
	query, args, err := squirrel.Select("id", "org_id", "version", "status", "tables", "deleted_at").
		From(dbmodels.TABLE_DATA_MODELS).
		Where(squirrel.Eq{"org_id": organizationID}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return models.DataModel{}, fmt.Errorf("squirrel.ToSql error: %w", err)
	}

	var model dbmodels.DbDataModel
	if err := db.pool.QueryRow(ctx, query, args...).
		Scan(&model.Id, &model.OrganizationId, &model.Version, &model.Status, &model.Tables, &model.DeletedAt); err != nil {
		return models.DataModel{}, fmt.Errorf("row.Scan error: %w", err)
	}
	return dbmodels.AdaptDataModel(model), nil
}

func (db *Database) DeleteDataModel(ctx context.Context, organizationID string) error {
	query, args, err := squirrel.Delete(dbmodels.TABLE_DATA_MODELS).
		Where(squirrel.Eq{"org_id": organizationID}).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("squirrel.ToSql error: %w", err)
	}

	if _, err := db.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("db.Exec error: %w", err)
	}
	return nil
}

func (db *Database) CreateDataModel(ctx context.Context, organizationID string, dataModel models.DataModel) error {
	tables, err := json.Marshal(dataModel.Tables)
	if err != nil {
		return fmt.Errorf("unable to marshal tables: %w", err)
	}

	query, args, err := squirrel.Insert(dbmodels.TABLE_DATA_MODELS).
		Columns("org_id", "version", "status", "tables").
		Values(organizationID, dataModel.Version, dataModel.Status.String(), tables).
		PlaceholderFormat(squirrel.Dollar).
		ToSql()

	if _, err := db.pool.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("db.Exec error: %w", err)
	}
	return nil
}
