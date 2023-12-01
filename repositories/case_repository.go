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

	query := selectCase().
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
		dbmodels.AdaptCaseWithContributorsAndTags,
	)
}

func (repo *MarbleDbRepository) GetCaseById(tx Transaction, caseId string) (models.Case, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(pgTx,
		selectCase().Where(squirrel.Eq{"c.id": caseId}),
		dbmodels.AdaptCaseWithContributorsAndTags,
	)
}

func (repo *MarbleDbRepository) CreateCase(tx Transaction, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_CASES).
			Columns(
				"id",
				"inbox_id",
				"name",
				"org_id",
			).
			Values(
				newCaseId,
				createCaseAttributes.InboxId,
				createCaseAttributes.Name,
				createCaseAttributes.OrganizationId,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateCase(tx Transaction, updateCaseAttributes models.UpdateCaseAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Update(dbmodels.TABLE_CASES).Where(squirrel.Eq{"id": updateCaseAttributes.Id})

	if updateCaseAttributes.InboxId != "" {
		query = query.Set("inbox_id", updateCaseAttributes.InboxId)
	}

	if updateCaseAttributes.Name != "" {
		query = query.Set("name", updateCaseAttributes.Name)
	}

	if updateCaseAttributes.Status != "" {
		query = query.Set("status", updateCaseAttributes.Status)
	}

	_, err := pgTx.ExecBuilder(query)
	return err
}

func (repo *MarbleDbRepository) CreateCaseTag(tx Transaction, newCaseTagId string, createCaseTagAttributes models.CreateCaseTagAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_CASE_TAGS).
			Columns(
				"id",
				"case_id",
				"tag_id",
				"deleted_at",
			).
			Values(
				newCaseTagId,
				createCaseTagAttributes.CaseId,
				createCaseTagAttributes.TagId,
				nil,
			),
	)
	return err
}

func (repo *MarbleDbRepository) SoftDeleteCaseTag(tx Transaction, tagId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Update(dbmodels.TABLE_CASE_TAGS).Where(squirrel.Eq{"id": tagId})
	query = query.Set("deleted_at", squirrel.Expr("NOW()"))

	_, err := pgTx.ExecBuilder(query)
	return err
}

func selectCase() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(pure_utils.WithPrefix(dbmodels.SelectCaseColumn, "c")...).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY cc.created_at) FILTER (WHERE cc.id IS NOT NULL) as contributors",
				strings.Join(pure_utils.WithPrefix(dbmodels.SelectCaseContributorColumn, "cc"), ","),
			),
		).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY ct.created_at) FILTER (WHERE ct.id IS NOT NULL) as tags",
				strings.Join(pure_utils.WithPrefix(dbmodels.SelectCaseTagColumn, "ct"), ","),
			),
		).
		Column("count(distinct d.id) as decisions_count").
		From(dbmodels.TABLE_CASES + " AS c").
		LeftJoin(dbmodels.TABLE_CASE_CONTRIBUTORS + " AS cc ON cc.case_id = c.id").
		LeftJoin(dbmodels.TABLE_CASE_TAGS + " AS ct ON ct.case_id = c.id AND ct.deleted_at IS NULL").
		LeftJoin(dbmodels.TABLE_DECISIONS + " AS d ON d.case_id = c.id").
		GroupBy("c.id").
		OrderBy("c.created_at DESC")
}
