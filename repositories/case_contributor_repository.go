package repositories

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetCaseContributor(tx Transaction, caseId, userId string) (*models.CaseContributor, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseContributorColumn...).
		From(dbmodels.TABLE_CASE_CONTRIBUTORS).
		Where("case_id = ?", caseId).
		Where("user_id = ?", userId)

	return SqlToOptionalModel(
		pgTx,
		query,
		dbmodels.AdaptCaseContributor,
	)
}

func (repo *MarbleDbRepository) CreateCaseContributor(tx Transaction, caseId, userId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Insert(dbmodels.TABLE_CASE_CONTRIBUTORS).
		Columns(
			"id",
			"case_id",
			"user_id",
		).
		Values(
			uuid.NewString(),
			caseId,
			userId,
		)

	_, err := pgTx.ExecBuilder(query)

	return err
}
