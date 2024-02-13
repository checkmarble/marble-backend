package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListOrganizationTags(ctx context.Context, tx Transaction_deprec, organizationId string, withCaseCount bool) ([]models.Tag, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)
	query := NewQueryBuilder().
		Select(dbmodels.SelectTagColumn...).
		From(fmt.Sprintf("%s AS t", dbmodels.TABLE_TAGS)).
		Where(squirrel.Eq{"org_id": organizationId}).
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("created_at DESC")

	if withCaseCount {
		query = query.Column("(SELECT count(distinct ct.case_id) FROM " + dbmodels.TABLE_CASE_TAGS + " AS ct WHERE ct.tag_id = t.id AND ct.deleted_at IS NULL) AS cases_count")
		return SqlToListOfModels(ctx, pgTx, query, dbmodels.AdaptTagWithCasesCount)
	}

	return SqlToListOfModels(ctx, pgTx, query, dbmodels.AdaptTag)
}

func (repo *MarbleDbRepository) CreateTag(ctx context.Context, tx Transaction_deprec, attributes models.CreateTagAttributes, newTagId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().Insert(dbmodels.TABLE_TAGS).
			Columns(
				"id",
				"org_id",
				"name",
				"color",
			).
			Values(
				newTagId,
				attributes.OrganizationId,
				attributes.Name,
				attributes.Color,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateTag(ctx context.Context, tx Transaction_deprec, attributes models.UpdateTagAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	query := NewQueryBuilder().Update(dbmodels.TABLE_TAGS).Where(squirrel.Eq{"id": attributes.TagId}).Set("updated_at", squirrel.Expr("NOW()"))

	if attributes.Color != "" {
		query = query.Set("color", attributes.Color)
	}
	if attributes.Name != "" {
		query = query.Set("name", attributes.Name)
	}
	_, err := pgTx.ExecBuilder(ctx, query)
	return err
}

func (repo *MarbleDbRepository) GetTagById(ctx context.Context, tx Transaction_deprec, tagId string) (models.Tag, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		NewQueryBuilder().Select(dbmodels.SelectTagColumn...).
			From(dbmodels.TABLE_TAGS).
			Where(squirrel.Eq{"deleted_at": nil}).
			Where(squirrel.Eq{"id": tagId}),
		dbmodels.AdaptTag,
	)
}

func (repo *MarbleDbRepository) SoftDeleteTag(ctx context.Context, tx Transaction_deprec, tagId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)
	query := NewQueryBuilder().Update(dbmodels.TABLE_TAGS).Where(squirrel.Eq{"id": tagId})
	query = query.Set("deleted_at", squirrel.Expr("NOW()"))
	query = query.Set("updated_at", squirrel.Expr("NOW()"))

	_, err := pgTx.ExecBuilder(ctx, query)
	return err
}
