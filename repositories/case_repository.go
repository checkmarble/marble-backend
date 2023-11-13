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
