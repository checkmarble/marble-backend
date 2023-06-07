package pg_repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"marble/marble-backend/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// TODO(data_model): handle versionning + status / change db schema if it's useless
type dbDataModel struct {
	ID        string      `db:"id"`
	OrgID     string      `db:"org_id"`
	Version   string      `db:"version"`
	Status    string      `db:"status"`
	Tables    []byte      `db:"tables"`
	DeletedAt pgtype.Time `db:"deleted_at"`
}

func (dm *dbDataModel) toDomain() (models.DataModel, error) {
	var tables map[models.TableName]models.Table
	if err := json.Unmarshal(dm.Tables, &tables); err != nil {
		return models.DataModel{}, fmt.Errorf("unable to unmarshal data model tables: %w", err)
	}
	return models.DataModel{
		Version: dm.Version,
		Status:  models.StatusFrom(dm.Status),
		Tables:  tables,
	}, nil
}

func (r *PGRepository) GetDataModel(ctx context.Context, orgID string) (models.DataModel, error) {
	sql, args, err := r.queryBuilder.
		Select("*").
		From("data_models").
		Where("org_id = ?", orgID).
		ToSql()
	if err != nil {
		return models.DataModel{}, fmt.Errorf("unable to build data model query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	dataModel, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbDataModel])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.DataModel{}, models.NotFoundInRepositoryError
	} else if err != nil {
		return models.DataModel{}, fmt.Errorf("unable to get data model for org(id: %s): %w", orgID, err)
	}

	return dataModel.toDomain()
}

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
	createdDataModel, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbDataModel])
	if err != nil {
		return models.DataModel{}, fmt.Errorf("unable to create token: %w", err)
	}

	return createdDataModel.toDomain()
}
