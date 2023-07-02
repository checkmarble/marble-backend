package pg_repository

import (
	"context"
	"encoding/json"
	"fmt"

	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/jackc/pgx/v5"
)

func (r *PGRepository) CreateDataModel(ctx context.Context, orgID string, dataModel models.DataModel) (models.DataModel, error) {
	tables, err := json.Marshal(dataModel.Tables)
	if err != nil {
		return models.DataModel{}, fmt.Errorf("unable to marshal tables: %w", err)
	}
	sql, args, err := r.queryBuilder.
		Insert("data_models").
		Columns(
			"org_id",
			"version",
			"status",
			"tables",
		).
		Values(
			orgID,
			dataModel.Version,
			dataModel.Status.String(),
			tables,
		).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return models.DataModel{}, fmt.Errorf("unable to build data model query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	createdDataModel, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbmodels.DbDataModel])
	if err != nil {
		return models.DataModel{}, fmt.Errorf("unable to create token: %w", err)
	}

	return dbmodels.AdaptDataModel(createdDataModel), nil
}
