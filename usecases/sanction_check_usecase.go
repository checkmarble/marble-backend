package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/pkg/errors"
)

type SanctionCheckProvider interface {
	Search(context.Context, models.SanctionCheckConfig,
		models.OpenSanctionsQuery) (models.SanctionCheckExecution, error)
}

type SanctionCheckRepository interface {
	InsertResults(context.Context, repositories.Executor, models.DecisionWithRuleExecutions) (models.SanctionCheckExecution, error)
}

type SanctionCheckUsecase struct {
	organizationRepository repositories.OrganizationRepository
	openSanctionsProvider  SanctionCheckProvider
	repository             SanctionCheckRepository
	executorFactory        executor_factory.ExecutorFactory
}

func (uc SanctionCheckUsecase) Execute(ctx context.Context, orgId string, cfg models.SanctionCheckConfig,
	query models.OpenSanctionsQuery,
) (models.SanctionCheckExecution, error) {
	org, err := uc.organizationRepository.GetOrganizationById(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return models.SanctionCheckExecution{},
			errors.Wrap(err, "could not retrieve organization")
	}

	query.OrgConfig = org.OpenSanctionsConfig

	matches, err := uc.openSanctionsProvider.Search(ctx, cfg, query)
	if err != nil {
		return models.SanctionCheckExecution{}, err
	}

	return matches, err
}

func (uc SanctionCheckUsecase) InsertResults(ctx context.Context,
	exec repositories.Executor,
	decision models.DecisionWithRuleExecutions,
) (models.SanctionCheckExecution, error) {
	return uc.repository.InsertResults(ctx, exec, decision)
}
