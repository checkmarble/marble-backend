package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) GetCaseContributor(ctx context.Context, tx Transaction_deprec, caseId, userId string) (*models.CaseContributor, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseContributorColumn...).
		From(dbmodels.TABLE_CASE_CONTRIBUTORS).
		Where("case_id = ?", caseId).
		Where("user_id = ?", userId)

	return SqlToOptionalModel(
		ctx,
		pgTx,
		query,
		dbmodels.AdaptCaseContributor,
	)
}

func (repo *MarbleDbRepository) CreateCaseContributor(ctx context.Context, tx Transaction_deprec, caseId, userId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	query := NewQueryBuilder().Insert(dbmodels.TABLE_CASE_CONTRIBUTORS).
		Columns(
			"case_id",
			"user_id",
		).
		Values(
			caseId,
			userId,
		)

	_, err := pgTx.ExecBuilder(ctx, query)

	return err
}
