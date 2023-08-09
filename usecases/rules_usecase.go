package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/scenarios"
	"marble/marble-backend/usecases/security"
)

type RuleUsecase struct {
	enforceSecurity           security.EnforceSecurityScenario
	repositoryLegacy          repositories.ScenarioIterationRuleRepositoryLegacy
	repository                repositories.RuleRepository
	scenarioFetcher           scenarios.ScenarioFetcher
	validateScenarioIteration scenarios.ValidateScenarioIteration
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

func (usecase *RuleUsecase) UpdateRule(ctx context.Context, organizationId string, rule models.UpdateRuleInput) (updatedRule models.Rule, validation models.ScenarioValidation, err error) {
	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, updatedRule.ScenarioIterationId)
	if err != nil {
		return updatedRule, validation, err
	}
	if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
		return updatedRule, validation, err
	}

	updatedRule, err = usecase.repositoryLegacy.UpdateRule(ctx, organizationId, rule)
	if err != nil {
		return updatedRule, validation, err
	}

	validation = usecase.validateScenarioIteration.Validate(scenarioAndIteration)
	return updatedRule, validation, err
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
	if err := usecase.enforceSecurity.CreateRule(scenarioAndIteration.Iteration); err != nil {
		return err
	}
	return usecase.repository.DeleteRule(ctx, ruleID)
}
