package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/scenarios"
)

type ScenarioIterationRuleUsecase struct {
	repository                repositories.ScenarioIterationRuleRepositoryLegacy
	scenarioFetcher           scenarios.ScenarioFetcher
	validateScenarioIteration scenarios.ValidateScenarioIteration
}

func (usecase *ScenarioIterationRuleUsecase) ListScenarioIterationRules(ctx context.Context, organizationId string, filters models.GetScenarioIterationRulesFilters) ([]models.Rule, error) {
	return usecase.repository.ListScenarioIterationRules(ctx, organizationId, filters)
}

func (usecase *ScenarioIterationRuleUsecase) CreateScenarioIterationRule(ctx context.Context, organizationId string, rule models.CreateRuleInput) (models.Rule, error) {
	return usecase.repository.CreateScenarioIterationRule(ctx, organizationId, rule)
}

func (usecase *ScenarioIterationRuleUsecase) GetScenarioIterationRule(ctx context.Context, organizationId string, ruleID string) (models.Rule, error) {
	return usecase.repository.GetScenarioIterationRule(ctx, organizationId, ruleID)
}

func (usecase *ScenarioIterationRuleUsecase) UpdateScenarioIterationRule(ctx context.Context, organizationId string, rule models.UpdateRuleInput) (updatedRule models.Rule, validation models.ScenarioValidation, err error) {

	updatedRule, err = usecase.repository.UpdateScenarioIterationRule(ctx, organizationId, rule)
	if err != nil {
		return updatedRule, validation, err
	}

	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, updatedRule.ScenarioIterationId)
	if err != nil {
		return updatedRule, validation, err
	}

	validation = usecase.validateScenarioIteration.Validate(scenarioAndIteration)
	return updatedRule, validation, err
}
