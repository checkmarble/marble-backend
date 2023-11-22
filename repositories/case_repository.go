package repositories

import (
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListOrganizationCases(tx Transaction, organizationId string, filters models.CaseFilters) ([]models.Case, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := selectJoinCaseAndContributors().
		Where(squirrel.Eq{"c.org_id": organizationId})

	if len(filters.Statuses) > 0 {
		query = query.Where(squirrel.Eq{"c.status": filters.Statuses})
	}

	if !filters.StartDate.IsZero() {
		query = query.Where(squirrel.GtOrEq{"c.created_at": filters.StartDate})
	}
	if !filters.EndDate.IsZero() {
		query = query.Where(squirrel.LtOrEq{"c.created_at": filters.EndDate})
	}

	return SqlToListOfModels(
		pgTx,
		query,
		dbmodels.AdaptCaseWithContributors,
	)
}

func (repo *MarbleDbRepository) GetCaseById(tx Transaction, caseId string) (models.Case, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(pgTx,
		selectJoinCaseAndContributors().Where(squirrel.Eq{"c.id": caseId}),
		dbmodels.AdaptCaseWithContributors,
	)
}

func (repo *MarbleDbRepository) CreateCase(tx Transaction, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_CASES).
			Columns(
				"id",
				"org_id",
				"name",
			).
			Values(
				newCaseId,
				createCaseAttributes.OrganizationId,
				createCaseAttributes.Name,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateCase(tx Transaction, updateCaseAttributes models.UpdateCaseAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Update(dbmodels.TABLE_CASES).Where(squirrel.Eq{"id": updateCaseAttributes.Id})

	if updateCaseAttributes.Name != "" {
		query = query.Set("name", updateCaseAttributes.Name)
	}

	if updateCaseAttributes.Status != "" {
		query = query.Set("status", updateCaseAttributes.Status)
	}

	_, err := pgTx.ExecBuilder(query)
	return err
}

func selectJoinCaseAndContributors() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(pure_utils.WithPrefix(dbmodels.SelectCaseColumn, "c")...).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY cc.created_at) FILTER (WHERE cc.id IS NOT NULL) as contributors",
				strings.Join(pure_utils.WithPrefix(dbmodels.SelectCaseContributorColumn, "cc"), ","),
			),
		).
		Column("count(distinct d.id) as decisions_count").
		From(dbmodels.TABLE_CASES + " AS c").
		LeftJoin(dbmodels.TABLE_CASE_CONTRIBUTORS + " AS cc ON cc.case_id = c.id").
		LeftJoin(dbmodels.TABLE_DECISIONS + " AS d ON d.case_id = c.id").
		GroupBy("c.id").
		OrderBy("c.created_at DESC")
}
