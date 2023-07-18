package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type ScenarioIterationRuleUsecase struct {
	repository repositories.ScenarioIterationRuleRepositoryLegacy
}

func (usecase *ScenarioIterationRuleUsecase) ListScenarioIterationRules(ctx context.Context, organizationID string, filters models.GetScenarioIterationRulesFilters) ([]models.Rule, error) {
	return usecase.repository.ListScenarioIterationRules(ctx, organizationID, filters)
}

func (usecase *ScenarioIterationRuleUsecase) CreateScenarioIterationRule(ctx context.Context, organizationID string, rule models.CreateRuleInput) (models.Rule, error) {
	return usecase.repository.CreateScenarioIterationRule(ctx, organizationID, rule)
}

func (usecase *ScenarioIterationRuleUsecase) GetScenarioIterationRule(ctx context.Context, organizationID string, ruleID string) (models.Rule, error) {
	return usecase.repository.GetScenarioIterationRule(ctx, organizationID, ruleID)
}

func (usecase *ScenarioIterationRuleUsecase) UpdateScenarioIterationRule(ctx context.Context, organizationID string, rule models.UpdateRuleInput) (models.Rule, error) {
	return usecase.repository.UpdateScenarioIterationRule(ctx, organizationID, rule)
}
