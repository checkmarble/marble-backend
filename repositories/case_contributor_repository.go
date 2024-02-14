package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) GetCaseContributor(ctx context.Context, exec Executor, caseId, userId string) (*models.CaseContributor, error) {
	exec = repo.executorGetter.ifNil(exec)

	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseContributorColumn...).
		From(dbmodels.TABLE_CASE_CONTRIBUTORS).
		Where("case_id = ?", caseId).
		Where("user_id = ?", userId)

	return SqlToOptionalModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptCaseContributor,
	)
}

func (repo *MarbleDbRepository) CreateCaseContributor(ctx context.Context, exec Executor, caseId, userId string) error {
	exec = repo.executorGetter.ifNil(exec)

	query := NewQueryBuilder().Insert(dbmodels.TABLE_CASE_CONTRIBUTORS).
		Columns(
			"case_id",
			"user_id",
		).
		Values(
			caseId,
			userId,
		)

	_, err := ExecBuilder(ctx, exec, query)

	return err
}
