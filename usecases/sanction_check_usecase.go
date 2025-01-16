package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type SanctionCheckProvider interface {
	Search(context.Context, models.SanctionCheckConfig, models.OpenSanctionsQuery) (models.SanctionCheckResult, error)
}

type SanctionCheckRepository interface {
	InsertResults(context.Context, models.SanctionCheckResult) (models.SanctionCheckResult, error)
}

type SanctionCheckUsecase struct {
	openSanctionsProvider SanctionCheckProvider
	repository            SanctionCheckRepository
}

func (uc SanctionCheckUsecase) Execute(ctx context.Context, cfg models.SanctionCheckConfig,
	query models.OpenSanctionsQuery,
) (models.SanctionCheckResult, error) {
	matches, err := uc.openSanctionsProvider.Search(ctx, cfg, query)
	if err != nil {
		return models.SanctionCheckResult{}, err
	}

	result, err := uc.repository.InsertResults(ctx, matches)

	return result, err
}
