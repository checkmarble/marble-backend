package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListOrganizationTags(ctx context.Context, exec Executor,
	organizationId string, target models.TagTarget, withCaseCount bool,
) ([]models.Tag, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SelectTagColumn...).
		From(fmt.Sprintf("%s AS t", dbmodels.TABLE_TAGS)).
		Where(squirrel.Eq{"org_id": organizationId}).
		Where(squirrel.Eq{"target": target}).
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("created_at DESC")

	if target == models.TagTargetCase && withCaseCount {
		query = query.Column("(SELECT count(distinct ct.case_id) FROM " +
			dbmodels.TABLE_CASE_TAGS + " AS ct WHERE ct.tag_id = t.id AND ct.deleted_at IS NULL) AS cases_count")
		return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptTagWithCasesCount)
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptTag)
}

func (repo *MarbleDbRepository) CreateTag(ctx context.Context, exec Executor,
	attributes models.CreateTagAttributes, newTagId string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_TAGS).
			Columns(
				"id",
				"org_id",
				"name",
				"color",
				"target",
			).
			Values(
				newTagId,
				attributes.OrganizationId,
				attributes.Name,
				attributes.Color,
				attributes.Target,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateTag(ctx context.Context, exec Executor, attributes models.UpdateTagAttributes) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_TAGS).Where(squirrel.Eq{
		"id": attributes.TagId,
	}).Set("updated_at", squirrel.Expr("NOW()"))

	if attributes.Color != "" {
		query = query.Set("color", attributes.Color)
	}
	if attributes.Name != "" {
		query = query.Set("name", attributes.Name)
	}
	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *MarbleDbRepository) GetTagById(ctx context.Context, exec Executor, tagId string) (models.Tag, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Tag{}, err
	}

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
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().Update(dbmodels.TABLE_TAGS).Where(squirrel.Eq{"id": tagId})
	query = query.Set("deleted_at", squirrel.Expr("NOW()"))
	query = query.Set("updated_at", squirrel.Expr("NOW()"))

	err := ExecBuilder(ctx, exec, query)
	return err
}
