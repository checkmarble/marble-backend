package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/analytics"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
)

type RuleUsecaseRepository interface {
	GetRuleById(tx repositories.Transaction, ruleId string) (models.Rule, error)
	ListRulesByIterationId(tx repositories.Transaction, iterationId string) ([]models.Rule, error)
	UpdateRule(tx repositories.Transaction, rule models.UpdateRuleInput) error
	DeleteRule(tx repositories.Transaction, ruleID string) error
	CreateRules(tx repositories.Transaction, rules []models.CreateRuleInput) ([]models.Rule, error)
	CreateRule(tx repositories.Transaction, rule models.CreateRuleInput) (models.Rule, error)
}

type RuleUsecase struct {
	organizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityScenario
	repository              RuleUsecaseRepository
	scenarioFetcher         scenarios.ScenarioFetcher
	transactionFactory      transaction.TransactionFactory
}

func (usecase *RuleUsecase) ListRules(iterationId string) ([]models.Rule, error) {
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.Rule, error) {
			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(tx, iterationId)
			if err != nil {
				return nil, err
			}
			if err := usecase.enforceSecurity.ReadScenarioIteration(scenarioAndIteration.Iteration); err != nil {
				return nil, err
			}
			return usecase.repository.ListRulesByIterationId(tx, iterationId)
		})
}

func (usecase *RuleUsecase) CreateRule(ctx context.Context, ruleInput models.CreateRuleInput) (models.Rule, error) {
	rule, err := transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Rule, error) {
			organizationId, err := usecase.organizationIdOfContext()
			if err != nil {
				return models.Rule{}, err
			}

			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(tx, ruleInput.ScenarioIterationId)
			if err != nil {
				return models.Rule{}, err
			}
			if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
				return models.Rule{}, err
			}
			// check if iteration is draft
			if scenarioAndIteration.Iteration.Version != nil {
				return models.Rule{}, fmt.Errorf("can't update rule as iteration %s is not in draft %w", scenarioAndIteration.Iteration.Id, models.ErrScenarioIterationNotDraft)
			}

			ruleInput.Id = utils.NewPrimaryKey(organizationId)
			_, err = usecase.repository.CreateRule(tx, ruleInput)
			if err != nil {
				return models.Rule{}, err
			}
			return usecase.repository.GetRuleById(tx, ruleInput.Id)
		})
	if err != nil {
		return models.Rule{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsRuleCreated, map[string]interface{}{"rule_id": ruleInput.Id})

	return rule, nil
}

func (usecase *RuleUsecase) GetRule(ruleId string) (models.Rule, error) {
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Rule, error) {
			rule, err := usecase.repository.GetRuleById(tx, ruleId)
			if err != nil {
				return models.Rule{}, err
			}

			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(tx, rule.ScenarioIterationId)
			if err != nil {
				return models.Rule{}, err
			}
			if err := usecase.enforceSecurity.ReadScenarioIteration(scenarioAndIteration.Iteration); err != nil {
				return models.Rule{}, err
			}
			return rule, nil
		})
}

func (usecase *RuleUsecase) UpdateRule(ctx context.Context, updateRule models.UpdateRuleInput) (updatedRule models.Rule, err error) {
	err = usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		rule, err := usecase.repository.GetRuleById(tx, updateRule.Id)
		if err != nil {
			return err
		}
		scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(tx, rule.ScenarioIterationId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
			return err
		}
		// check if iteration is draft
		if scenarioAndIteration.Iteration.Version != nil {
			return fmt.Errorf("can't update rule as iteration %s is not in draft %w", scenarioAndIteration.Iteration.Id, models.ErrScenarioIterationNotDraft)
		}

		err = usecase.repository.UpdateRule(tx, updateRule)
		if err != nil {
			return err
		}

		updatedRule, err = usecase.repository.GetRuleById(tx, updateRule.Id)
		return err
	})

	if err != nil {
		return models.Rule{}, err
	}

	analytics.TrackEvent(ctx, models.AnalyticsRuleUpdated, map[string]interface{}{"rule_id": updateRule.Id})

	return updatedRule, err
}

func (usecase *RuleUsecase) DeleteRule(ctx context.Context, ruleId string) error {
	err := usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		rule, err := usecase.repository.GetRuleById(tx, ruleId)
		if err != nil {
			return err
		}

		scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(tx, rule.ScenarioIterationId)
		if err != nil {
			return err
		}
		if scenarioAndIteration.Iteration.Version != nil {
			return fmt.Errorf("can't delete rule as iteration %s is not in draft", scenarioAndIteration.Iteration.Id)
		}
		if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
			return err
		}
		return usecase.repository.DeleteRule(tx, ruleId)
	})
	if err != nil {
		return err
	}

	analytics.TrackEvent(ctx, models.AnalyticsRuleDeleted, map[string]interface{}{"rule_id": ruleId})

	return nil
}
