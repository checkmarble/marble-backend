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
	AnySnoozesForIteration(
		ctx context.Context,
		exec repositories.Executor,
		snoozeGroupIds []string,
	) (map[string]bool, error)
	CreateRuleSnooze(
		ctx context.Context,
		exec repositories.Executor,
		input models.RuleSnoozeCreateInput,
	) error
}

type enforceSecuritySnoozes interface {
	ReadSnoozesOfDecision(ctx context.Context, decision models.Decision) error
	ReadSnoozesOfIteration(ctx context.Context, iteration models.ScenarioIteration) error
}

type RuleSnoozeUsecase struct {
	decisionGetter       decisionGetter
	executorFactory      executor_factory.ExecutorFactory
	iterationGetter      iterationGetter
	ruleSnoozeRepository ruleSnoozeRepository
	enforceSecurity      enforceSecuritySnoozes
}

func NewRuleSnoozeUsecase(
	d decisionGetter, e executor_factory.ExecutorFactory, i iterationGetter, s ruleSnoozeRepository, es enforceSecuritySnoozes,
) RuleSnoozeUsecase {
	return RuleSnoozeUsecase{
		decisionGetter:       d,
		executorFactory:      e,
		iterationGetter:      i,
		ruleSnoozeRepository: s,
		enforceSecurity:      es,
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

	if err := usecase.enforceSecurity.ReadSnoozesOfDecision(ctx, decisions[0]); err != nil {
		return models.SnoozesOfDecision{}, err
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

func (usecase RuleSnoozeUsecase) ActiveSnoozesForScenarioIteration(ctx context.Context, iterationId string) (models.SnoozesOfIteration, error) {
	exec := usecase.executorFactory.NewExecutor()
	it, err := usecase.iterationGetter.GetScenarioIteration(ctx, exec, iterationId)
	if err != nil {
		return models.SnoozesOfIteration{}, err
	}

	if err := usecase.enforceSecurity.ReadSnoozesOfIteration(ctx, it); err != nil {
		return models.SnoozesOfIteration{}, err
	}

	snooze_group_ids := make([]string, 0, len(it.Rules))
	for _, rule := range it.Rules {
		if rule.SnoozeGroupId != nil {
			snooze_group_ids = append(snooze_group_ids, *rule.SnoozeGroupId)
		}
	}
	snoozesByRule, err := usecase.ruleSnoozeRepository.AnySnoozesForIteration(
		ctx, exec, snooze_group_ids)
	if err != nil {
		return models.SnoozesOfIteration{}, err
	}

	snoozes := make([]models.RuleSnoozeInformation, 0, len(it.Rules))
	for _, rule := range it.Rules {
		if rule.SnoozeGroupId == nil {
			snoozes = append(snoozes, models.RuleSnoozeInformation{
				RuleId:           rule.Id,
				SnoozeGroupId:    "",
				HasSnoozesActive: false,
			})
			continue
		}
		hasSnoozesActive, ok := snoozesByRule[*rule.SnoozeGroupId]
		if !ok {
			hasSnoozesActive = false
		}
		snoozes = append(snoozes, models.RuleSnoozeInformation{
			RuleId:           rule.Id,
			SnoozeGroupId:    *rule.SnoozeGroupId,
			HasSnoozesActive: hasSnoozesActive,
		})
	}

	return models.SnoozesOfIteration{
		IterationId: iterationId,
		RuleSnoozes: snoozes,
	}, nil
}
