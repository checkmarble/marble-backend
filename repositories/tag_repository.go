package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListOrganizationTags(ctx context.Context, exec Executor, organizationId string, withCaseCount bool) ([]models.Tag, error) {
	exec = repo.executorGetter.ifNil(exec)
	query := NewQueryBuilder().
		Select(dbmodels.SelectTagColumn...).
		From(fmt.Sprintf("%s AS t", dbmodels.TABLE_TAGS)).
		Where(squirrel.Eq{"org_id": organizationId}).
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("created_at DESC")

	if withCaseCount {
		query = query.Column("(SELECT count(distinct ct.case_id) FROM " + dbmodels.TABLE_CASE_TAGS + " AS ct WHERE ct.tag_id = t.id AND ct.deleted_at IS NULL) AS cases_count")
		return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptTagWithCasesCount)
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptTag)
}

func (repo *MarbleDbRepository) CreateTag(ctx context.Context, exec Executor, attributes models.CreateTagAttributes, newTagId string) error {
	exec = repo.executorGetter.ifNil(exec)

	_, err := ExecBuilder(
		ctx,
		exec,
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

func (repo *MarbleDbRepository) UpdateTag(ctx context.Context, exec Executor, attributes models.UpdateTagAttributes) error {
	exec = repo.executorGetter.ifNil(exec)

	query := NewQueryBuilder().Update(dbmodels.TABLE_TAGS).Where(squirrel.Eq{"id": attributes.TagId}).Set("updated_at", squirrel.Expr("NOW()"))

	if attributes.Color != "" {
		query = query.Set("color", attributes.Color)
	}
	if attributes.Name != "" {
		query = query.Set("name", attributes.Name)
	}
	_, err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *MarbleDbRepository) GetTagById(ctx context.Context, exec Executor, tagId string) (models.Tag, error) {
	exec = repo.executorGetter.ifNil(exec)

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().Select(dbmodels.SelectTagColumn...).
			From(dbmodels.TABLE_TAGS).
			Where(squirrel.Eq{"deleted_at": nil}).
			Where(squirrel.Eq{"id": tagId}),
		dbmodels.AdaptTag,
	)
}

func (repo *MarbleDbRepository) SoftDeleteTag(ctx context.Context, exec Executor, tagId string) error {
	exec = repo.executorGetter.ifNil(exec)
	query := NewQueryBuilder().Update(dbmodels.TABLE_TAGS).Where(squirrel.Eq{"id": tagId})
	query = query.Set("deleted_at", squirrel.Expr("NOW()"))
	query = query.Set("updated_at", squirrel.Expr("NOW()"))

	_, err := ExecBuilder(ctx, exec, query)
	return err
}
