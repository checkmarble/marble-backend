package repositories

import (
	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListOrganizationCases(tx Transaction, organizationId string) ([]models.Case, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseColumn...).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{"org_id": organizationId}).
		OrderBy("created_at DESC")

	return SqlToListOfModels(
		pgTx,
		query,
		dbmodels.AdaptCase,
	)
}

func (repo *MarbleDbRepository) GetCaseById(tx Transaction, caseId string) (models.Case, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	decisions, err := SqlToListOfModels(pgTx,
		NewQueryBuilder().Select(dbmodels.SelectDecisionColumn...).
			From(dbmodels.TABLE_DECISIONS).
			Where(squirrel.Eq{"case_id": caseId}).
			OrderBy("created_at DESC"),
		func(dbDecision dbmodels.DbDecision) (models.Decision, error) {
			return dbmodels.AdaptDecision(dbDecision, []models.RuleExecution{}), nil
		},
	)
	
	c, err := SqlToModel(pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseColumn...).
			From(dbmodels.TABLE_CASES).
			Where(squirrel.Eq{"id": caseId}),
		func(dbCase dbmodels.DBCase) (models.Case, error) {
			return dbmodels.AdaptCaseExtended(dbCase, decisions), nil
		},
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
