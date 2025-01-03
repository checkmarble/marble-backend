package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListFeatures(ctx context.Context, exec Executor) ([]models.Feature, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SelectFeatureColumn...).
		From(fmt.Sprintf("%s AS t", dbmodels.TABLE_FEATURES)).
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptFeature)
}

func (repo *MarbleDbRepository) CreateFeature(ctx context.Context, exec Executor,
	attributes models.CreateFeatureAttributes, newFeatureId string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_FEATURES).
			Columns(dbmodels.SelectFeatureColumn...).
			Values(newFeatureId, attributes.Name),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateFeature(ctx context.Context, exec Executor, attributes models.UpdateFeatureAttributes) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_FEATURES).Where(squirrel.Eq{
		"id": attributes.Id,
	}).Set("updated_at", squirrel.Expr("NOW()"))

	if attributes.Name != "" {
		query = query.Set("name", attributes.Name)
	}
	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *MarbleDbRepository) GetFeatureById(ctx context.Context, exec Executor, featureId string) (models.Feature, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Feature{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().Select(dbmodels.SelectFeatureColumn...).
			From(dbmodels.TABLE_FEATURES).
			Where(squirrel.Eq{"deleted_at": nil}).
			Where(squirrel.Eq{"id": featureId}),
		dbmodels.AdaptFeature,
	)
}

func (repo *MarbleDbRepository) SoftDeleteFeature(ctx context.Context, exec Executor, featureId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().Update(dbmodels.TABLE_FEATURES).Where(squirrel.Eq{"id": featureId})
	query = query.Set("deleted_at", squirrel.Expr("NOW()"))
	query = query.Set("updated_at", squirrel.Expr("NOW()"))

	err := ExecBuilder(ctx, exec, query)
	return err
}
