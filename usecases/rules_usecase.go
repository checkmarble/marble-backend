package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/scenarios"
	"marble/marble-backend/usecases/security"
)

type RuleUsecase struct {
	enforceSecurity    security.EnforceSecurityScenario
	repositoryLegacy   repositories.ScenarioIterationRuleRepositoryLegacy
	repository         repositories.RuleRepository
	scenarioFetcher    scenarios.ScenarioFetcher
	transactionFactory repositories.TransactionFactory
}

func (usecase *RuleUsecase) ListRules(ctx context.Context, organizationId string, iterationId string) ([]models.Rule, error) {
	return repositories.TransactionReturnValue(
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

func (usecase *RuleUsecase) CreateRule(ctx context.Context, organizationId string, rule models.CreateRuleInput) (models.Rule, error) {
	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, rule.ScenarioIterationId)
	if err != nil {
		return models.Rule{}, err
	}
	if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
		return models.Rule{}, err
	}
	return usecase.repositoryLegacy.CreateRule(ctx, organizationId, rule)
}

func (usecase *RuleUsecase) GetRule(ctx context.Context, ruleId string) (models.Rule, error) {
	return repositories.TransactionReturnValue(
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

func (usecase *RuleUsecase) UpdateRule(ctx context.Context, organizationId string, updateRule models.UpdateRuleInput) (updatedRule models.Rule, err error) {
	rule, err := usecase.repository.GetRuleById(nil, updateRule.Id)
	if err != nil {
		return updatedRule, err
	}
	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, rule.ScenarioIterationId)
	if err != nil {
		return updatedRule, err
	}
	if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
		return updatedRule, err
	}

	updatedRule, err = usecase.repositoryLegacy.UpdateRule(ctx, organizationId, updateRule)
	if err != nil {
		return updatedRule, err
	}

	return updatedRule, err
}

func (usecase *RuleUsecase) DeleteRule(ruleId string) error {
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
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
}
