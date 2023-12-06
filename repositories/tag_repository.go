package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListOrganizationTags(tx Transaction, organizationId string) ([]models.Tag, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	query := NewQueryBuilder().
		Select(dbmodels.SelectTagColumn...).
		From(dbmodels.TABLE_TAGS).
		Where(squirrel.Eq{"org_id": organizationId}).
		Where(squirrel.Eq{"deleted_at": nil}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(pgTx, query, dbmodels.AdaptTag)
}

func (repo *MarbleDbRepository) CreateTag(tx Transaction, attributes models.CreateTagAttributes, newTagId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
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

func (repo *MarbleDbRepository) UpdateTag(tx Transaction, attributes models.UpdateTagAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Update(dbmodels.TABLE_TAGS).Where(squirrel.Eq{"id": attributes.TagId}).Set("updated_at", squirrel.Expr("NOW()"))

	if attributes.Color != "" {
		query = query.Set("color", attributes.Color)
	}
	_, err := pgTx.ExecBuilder(query)
	return err
}

func (repo *MarbleDbRepository) GetTagById(tx Transaction, tagId string) (models.Tag, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(pgTx,
		NewQueryBuilder().Select(dbmodels.SelectTagColumn...).
			From(dbmodels.TABLE_TAGS).
			Where(squirrel.Eq{"deleted_at": nil}).
			Where(squirrel.Eq{"id": tagId}),
		dbmodels.AdaptTag,
	)
}

func (repo *MarbleDbRepository) SoftDeleteTag(tx Transaction, tagId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)
	query := NewQueryBuilder().Update(dbmodels.TABLE_TAGS).Where(squirrel.Eq{"id": tagId})
	query = query.Set("deleted_at", squirrel.Expr("NOW()"))
	query = query.Set("updated_at", squirrel.Expr("NOW()"))

	_, err := pgTx.ExecBuilder(query)
	return err
}