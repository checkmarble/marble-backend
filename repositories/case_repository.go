package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListOrganizationCases(tx Transaction, organizationId string, filters models.CaseFilters) ([]models.Case, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseColumn...).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{"org_id": organizationId}).
		OrderBy("created_at DESC")

	if len(filters.Statuses) > 0 {
		query = query.Where(squirrel.Eq{"status": filters.Statuses})
	}

	if !filters.StartDate.IsZero() {
		query = query.Where(squirrel.GtOrEq{"created_at": filters.StartDate})
	}
	if !filters.EndDate.IsZero() {
		query = query.Where(squirrel.LtOrEq{"created_at": filters.EndDate})
	}

	return SqlToListOfModels(
		pgTx,
		query,
		dbmodels.AdaptCase,
	)
}

func (repo *MarbleDbRepository) GetCaseById(tx Transaction, caseId string) (models.Case, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	c, err := SqlToModel(pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseColumn...).
			From(dbmodels.TABLE_CASES).
			Where(squirrel.Eq{"id": caseId}),
		dbmodels.AdaptCase,
	)

	if err != nil {
		return models.Case{}, err
	}
	return c, nil
}

func (repo *MarbleDbRepository) CreateCase(tx Transaction, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_CASES).
			Columns(
				"id",
				"org_id",
				"name",
				"description",
			).
			Values(
				newCaseId,
				createCaseAttributes.OrganizationId,
				createCaseAttributes.Name,
				createCaseAttributes.Description,
			),
	)
	return err
}
