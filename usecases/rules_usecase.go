package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

type RuleUsecaseRepository interface {
	GetRuleById(ctx context.Context, exec repositories.Executor, ruleId string) (models.Rule, error)
	ListRulesByIterationId(ctx context.Context, exec repositories.Executor, iterationId string) ([]models.Rule, error)
	UpdateRule(ctx context.Context, exec repositories.Executor, rule models.UpdateRuleInput) error
	DeleteRule(ctx context.Context, exec repositories.Executor, ruleID string) error
	CreateRules(ctx context.Context, exec repositories.Executor, rules []models.CreateRuleInput) ([]models.Rule, error)
	CreateRule(ctx context.Context, exec repositories.Executor, rule models.CreateRuleInput) (models.Rule, error)
}

type RuleUsecase struct {
	organizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityScenario
	repository              RuleUsecaseRepository
	scenarioFetcher         scenarios.ScenarioFetcher
	transactionFactory      executor_factory.TransactionFactory
}

func (usecase *RuleUsecase) ListRules(ctx context.Context, iterationId string) ([]models.Rule, error) {
	return executor_factory.TransactionReturnValue(ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) ([]models.Rule, error) {
			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, iterationId)
			if err != nil {
				return nil, err
			}
			if err := usecase.enforceSecurity.ReadScenarioIteration(
				scenarioAndIteration.Iteration); err != nil {
				return nil, err
			}
			return usecase.repository.ListRulesByIterationId(ctx, tx, iterationId)
		})
}

func (usecase *RuleUsecase) CreateRule(ctx context.Context, ruleInput models.CreateRuleInput) (models.Rule, error) {
	rule, err := executor_factory.TransactionReturnValue(ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.Rule, error) {
			organizationId, err := usecase.organizationIdOfContext()
			if err != nil {
				return models.Rule{}, err
			}

			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, ruleInput.ScenarioIterationId)
			if err != nil {
				return models.Rule{}, err
			}
			if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
				return models.Rule{}, err
			}
			// check if iteration is draft
			if scenarioAndIteration.Iteration.Version != nil {
				return models.Rule{}, errors.Wrap(
					models.ErrScenarioIterationNotDraft,
					fmt.Sprintf("can't update rule as iteration %s is not in draft", scenarioAndIteration.Iteration.Id))
			}

			ruleInput.Id = utils.NewPrimaryKey(organizationId)
			_, err = usecase.repository.CreateRule(ctx, tx, ruleInput)
			if err != nil {
				return models.Rule{}, err
			}
			return usecase.repository.GetRuleById(ctx, tx, ruleInput.Id)
		})
	if err != nil {
		return models.Rule{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsRuleCreated, map[string]interface{}{
		"rule_id": ruleInput.Id,
	})

	return rule, nil
}

func (usecase *RuleUsecase) GetRule(ctx context.Context, ruleId string) (models.Rule, error) {
	return executor_factory.TransactionReturnValue(ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.Rule, error) {
			rule, err := usecase.repository.GetRuleById(ctx, tx, ruleId)
			if err != nil {
				return models.Rule{}, err
			}

			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, rule.ScenarioIterationId)
			if err != nil {
				return models.Rule{}, err
			}
			if err := usecase.enforceSecurity.ReadScenarioIteration(
				scenarioAndIteration.Iteration); err != nil {
				return models.Rule{}, err
			}
			return rule, nil
		})
}

func (usecase *RuleUsecase) UpdateRule(ctx context.Context, updateRule models.UpdateRuleInput) (updatedRule models.Rule, err error) {
	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		rule, err := usecase.repository.GetRuleById(ctx, tx, updateRule.Id)
		if err != nil {
			return err
		}
		scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, rule.ScenarioIterationId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
			return err
		}
		// check if iteration is draft
		if scenarioAndIteration.Iteration.Version != nil {
			return errors.Wrap(
				models.ErrScenarioIterationNotDraft,
				fmt.Sprintf("can't update rule as iteration %s is not in draft", scenarioAndIteration.Iteration.Id),
			)
		}

		err = usecase.repository.UpdateRule(ctx, tx, updateRule)
		if err != nil {
			return err
		}

		updatedRule, err = usecase.repository.GetRuleById(ctx, tx, updateRule.Id)
		return err
	})
	if err != nil {
		return models.Rule{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsRuleUpdated, map[string]interface{}{
		"rule_id": updateRule.Id,
	})

	return updatedRule, err
}

func (usecase *RuleUsecase) DeleteRule(ctx context.Context, ruleId string) error {
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		rule, err := usecase.repository.GetRuleById(ctx, tx, ruleId)
		if err != nil {
			return err
		}

		scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, rule.ScenarioIterationId)
		if err != nil {
			return err
		}
		if scenarioAndIteration.Iteration.Version != nil {
			return fmt.Errorf("can't delete rule as iteration %s is not in draft", scenarioAndIteration.Iteration.Id)
		}
		if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
			return err
		}
		return usecase.repository.DeleteRule(ctx, tx, ruleId)
	})
	if err != nil {
		return err
	}

	tracking.TrackEvent(ctx, models.AnalyticsRuleDeleted, map[string]interface{}{
		"rule_id": ruleId,
	})

	return nil
}
