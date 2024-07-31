package usecases

import (
	"context"

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

type ruleSnoozeRepository interface {
	CreateSnoozeGroup(ctx context.Context, exec repositories.Executor, id, organizationId string) error
	ListRuleSnoozesForDecision(
		ctx context.Context,
		exec repositories.Executor,
		snoozeGroupIds []string,
		pivotValue string,
	) ([]models.RuleSnooze, error)
	CreateRuleSnooze(
		ctx context.Context,
		exec repositories.Executor,
		input models.RuleSnoozeCreateInput,
	) error
}

type RuleSnoozeUsecase struct {
	decisionGetter       decisionGetter
	executorFactory      executor_factory.ExecutorFactory
	iterationGetter      iterationGetter
	ruleSnoozeRepository ruleSnoozeRepository
}

func NewRuleSnoozeUsecase(d decisionGetter, e executor_factory.ExecutorFactory, i iterationGetter, s ruleSnoozeRepository) RuleSnoozeUsecase {
	return RuleSnoozeUsecase{
		decisionGetter:       d,
		executorFactory:      e,
		iterationGetter:      i,
		ruleSnoozeRepository: s,
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

	if decisions[0].PivotValue == nil {
		// no snoozes possible if decision doesn't have a pivot value
		return models.SnoozesOfDecision{
			DecisionId:  decisionId,
			RuleSnoozes: make([]models.RuleSnoozeWithRuleId, 0),
			Iteration:   it,
		}, nil
	}

	snooze_group_ids := make([]string, 0, len(it.Rules))
	for _, rule := range it.Rules {
		if rule.SnoozeGroupId != nil {
			snooze_group_ids = append(snooze_group_ids, *rule.SnoozeGroupId)
		}
	}

	snoozes, err := usecase.ruleSnoozeRepository.ListRuleSnoozesForDecision(
		ctx, exec, snooze_group_ids, *decisions[0].PivotValue)
	if err != nil {
		return models.SnoozesOfDecision{}, err
	}

	return models.NewSnoozesOfDecision(decisionId, snoozes, it), nil
}
