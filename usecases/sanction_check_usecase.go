package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/pkg/errors"
)

type SanctionCheckProvider interface {
	Search(context.Context, models.OrganizationOpenSanctionsConfig, models.SanctionCheckConfig,
		models.OpenSanctionsQuery) (models.SanctionCheckResult, error)
}

type SanctionCheckRepository interface {
	InsertResults(context.Context, models.SanctionCheckResult) (models.SanctionCheckResult, error)
}

type SanctionCheckUsecase struct {
	organizationRepository repositories.OrganizationRepository
	openSanctionsProvider  SanctionCheckProvider
	repository             SanctionCheckRepository
	executorFactory        executor_factory.ExecutorFactory
}

func (uc SanctionCheckUsecase) Execute(ctx context.Context, orgId string, cfg models.SanctionCheckConfig,
	query models.OpenSanctionsQuery,
) (models.SanctionCheckResult, error) {
	org, err := uc.organizationRepository.GetOrganizationById(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return models.SanctionCheckResult{}, errors.Wrap(err, "could not retrieve organization")
	}

	matches, err := uc.openSanctionsProvider.Search(ctx, org.OpenSanctionsConfig, cfg, query)
	if err != nil {
		return models.SanctionCheckResult{}, err
	}

	result, err := uc.repository.InsertResults(ctx, matches)

	return result, err
}
