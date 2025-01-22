package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/pkg/errors"
)

type SanctionCheckProvider interface {
	Search(context.Context, models.SanctionCheckConfig,
		models.OpenSanctionsQuery) (models.SanctionCheck, error)
}

type SanctionCheckDecisionRepository interface {
	DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error)
}

type SanctionCheckRepository interface {
	ListSanctionChecksForDecision(context.Context, repositories.Executor, string) ([]models.SanctionCheck, error)
	ListSanctionCheckMatches(ctx context.Context, exec repositories.Executor, sanctionCheckId string) (
		[]models.SanctionCheckMatch, error)
	InsertSanctionCheck(context.Context, repositories.Executor,
		models.DecisionWithRuleExecutions) (models.SanctionCheck, error)
}

type SanctionCheckUsecase struct {
	enforceSecurityDecision security.EnforceSecurityDecision

	organizationRepository repositories.OrganizationRepository
	decisionRepository     SanctionCheckDecisionRepository
	openSanctionsProvider  SanctionCheckProvider
	repository             SanctionCheckRepository
	executorFactory        executor_factory.ExecutorFactory
}

func (uc SanctionCheckUsecase) ListSanctionChecks(ctx context.Context, decisionId string) ([]models.SanctionCheck, error) {
	decision, err := uc.decisionRepository.DecisionsById(ctx,
		uc.executorFactory.NewExecutor(), []string{decisionId})
	if err != nil {
		return nil, err
	}
	if len(decision) == 0 {
		return nil, errors.Wrap(models.NotFoundError, "requested decision does not exist")
	}

	if err := uc.enforceSecurityDecision.ReadDecision(decision[0]); err != nil {
		return nil, err
	}

	sanctionChecks, err := uc.repository.ListSanctionChecksForDecision(ctx,
		uc.executorFactory.NewExecutor(), decision[0].DecisionId)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve sanction check")
	}

	// TODO: anything supports nested queries?
	for idx, sc := range sanctionChecks {
		matches, err := uc.repository.ListSanctionCheckMatches(ctx,
			uc.executorFactory.NewExecutor(), sc.Id)
		if err != nil {
			return nil, errors.Wrap(err, "could not retrieve sanction check matches")
		}

		sanctionChecks[idx].Count = len(matches)
		sanctionChecks[idx].Matches = matches
	}

	return sanctionChecks, nil
}

func (uc SanctionCheckUsecase) Execute(ctx context.Context, orgId string, cfg models.SanctionCheckConfig,
	query models.OpenSanctionsQuery,
) (models.SanctionCheck, error) {
	org, err := uc.organizationRepository.GetOrganizationById(ctx,
		uc.executorFactory.NewExecutor(), orgId)
	if err != nil {
		return models.SanctionCheck{},
			errors.Wrap(err, "could not retrieve organization")
	}

	query.OrgConfig = org.OpenSanctionsConfig

	matches, err := uc.openSanctionsProvider.Search(ctx, cfg, query)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	return matches, err
}

func (uc SanctionCheckUsecase) InsertResults(ctx context.Context,
	exec repositories.Executor,
	decision models.DecisionWithRuleExecutions,
) (models.SanctionCheck, error) {
	return uc.repository.InsertSanctionCheck(ctx, exec, decision)
}
