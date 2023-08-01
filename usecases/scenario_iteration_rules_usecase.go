package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type ScenarioIterationRuleUsecase struct {
	repository repositories.ScenarioIterationRuleRepositoryLegacy
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

func (usecase *ScenarioIterationRuleUsecase) UpdateScenarioIterationRule(ctx context.Context, organizationId string, rule models.UpdateRuleInput) (models.Rule, error) {
	return usecase.repository.UpdateScenarioIterationRule(ctx, organizationId, rule)
}
