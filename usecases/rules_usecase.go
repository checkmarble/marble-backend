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
	enforceSecurity  security.EnforceSecurityScenario
	repositoryLegacy repositories.ScenarioIterationRuleRepositoryLegacy
	repository       repositories.RuleRepository
	scenarioFetcher  scenarios.ScenarioFetcher
}

func (usecase *RuleUsecase) ListRules(ctx context.Context, organizationId string, filters models.GetRulesFilters) ([]models.Rule, error) {
	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, filters.ScenarioIterationId)
	if err != nil {
		return nil, err
	}
	if err := usecase.enforceSecurity.ReadScenarioIteration(scenarioAndIteration.Iteration); err != nil {
		return nil, err
	}
	return usecase.repositoryLegacy.ListRules(ctx, organizationId, filters)
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

func (usecase *RuleUsecase) GetRule(ctx context.Context, organizationId string, ruleID string) (models.Rule, error) {
	rule, err := usecase.repositoryLegacy.GetRule(ctx, organizationId, ruleID)
	if err != nil {
		return models.Rule{}, err
	}

	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, rule.ScenarioIterationId)
	if err != nil {
		return models.Rule{}, err
	}
	if err := usecase.enforceSecurity.ReadScenarioIteration(scenarioAndIteration.Iteration); err != nil {
		return models.Rule{}, err
	}
	return rule, nil
}

func (usecase *RuleUsecase) UpdateRule(ctx context.Context, organizationId string, updateRule models.UpdateRuleInput) (updatedRule models.Rule, err error) {
	rule, err := usecase.repositoryLegacy.GetRule(ctx, organizationId, updateRule.Id)
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

func (usecase *RuleUsecase) DeleteRule(ctx context.Context, organizationId string, ruleID string) error {
	rule, err := usecase.repositoryLegacy.GetRule(ctx, organizationId, ruleID)
	if err != nil {
		return err
	}

	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, rule.ScenarioIterationId)
	if err != nil {
		return err
	}
	if scenarioAndIteration.Iteration.Version != nil {
		return fmt.Errorf("can't delete rule as iteration %s is not in draft", scenarioAndIteration.Iteration.Id)
	}
	if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
		return err
	}
	return usecase.repository.DeleteRule(ctx, ruleID)
}
