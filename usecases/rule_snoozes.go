package usecases

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/pkg/errors"
)

type decisionGetter interface {
	DecisionsById(ctx context.Context, exec repositories.Executor, decisionIds []string) ([]models.Decision, error)
}
type iterationGetter interface {
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)
}

type RuleSnoozeUsecase struct {
	decisionGetter  decisionGetter
	executorFactory executor_factory.ExecutorFactory
	iterationGetter iterationGetter
}

func NewRuleSnoozeUsecase(d decisionGetter, e executor_factory.ExecutorFactory, i iterationGetter) RuleSnoozeUsecase {
	return RuleSnoozeUsecase{
		decisionGetter:  d,
		executorFactory: e,
		iterationGetter: i,
	}
}

func (usecase RuleSnoozeUsecase) ActiveSnoozesForDecision(ctx context.Context, decisionId string) (models.SnoozesOfDecision, error) {
	exec := usecase.executorFactory.NewExecutor()
	decisions, err := usecase.decisionGetter.DecisionsById(ctx, exec, []string{decisionId})
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}
	if len(decisions) == 0 {
		return models.SnoozesOfDecision{}, errors.Wrapf(models.NotFoundError, "decision %s not found", decisionId)
	}

	it, err := usecase.iterationGetter.GetScenarioIteration(ctx, exec, decisions[0].ScenarioIterationId)
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}

	snoozes := models.NewSnoozesOfDecision(decisionId,
		[]models.RuleSnooze{
			{
				Id: "1", SnoozeGroupId: "", PivotValue: "1", StartsAt: time.Now(), ExpiresAt: time.Now(), CreatedByUser: "1",
			},
			{
				Id: "2", SnoozeGroupId: "", PivotValue: "2", StartsAt: time.Now(), ExpiresAt: time.Now(), CreatedByUser: "2",
			},
			{
				Id: "3", SnoozeGroupId: "", PivotValue: "3", StartsAt: time.Now(), ExpiresAt: time.Now(), CreatedByUser: "3",
			},
		},
		it,
	)
	return snoozes, nil
}
