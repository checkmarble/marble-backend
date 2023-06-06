package usecases

import (
	"context"
	"marble/marble-backend/app"
	"marble/marble-backend/repositories"
)

type ScenarioIterationRuleUsecase struct {
	repository repositories.ScenarioIterationRuleRepository
}

func (usecase *ScenarioIterationRuleUsecase) ListScenarioIterationRules(ctx context.Context, organizationID string, filters app.GetScenarioIterationRulesFilters) ([]app.Rule, error) {
	return usecase.repository.ListScenarioIterationRules(ctx, organizationID, filters)
}

func (usecase *ScenarioIterationRuleUsecase) CreateScenarioIterationRule(ctx context.Context, organizationID string, rule app.CreateRuleInput) (app.Rule, error) {
	return usecase.repository.CreateScenarioIterationRule(ctx, organizationID, rule)
}

func (usecase *ScenarioIterationRuleUsecase) GetScenarioIterationRule(ctx context.Context, organizationID string, ruleID string) (app.Rule, error) {
	return usecase.repository.GetScenarioIterationRule(ctx, organizationID, ruleID)
}

func (usecase *ScenarioIterationRuleUsecase) UpdateScenarioIterationRule(ctx context.Context, organizationID string, rule app.UpdateRuleInput) (app.Rule, error) {
	return usecase.repository.UpdateScenarioIterationRule(ctx, organizationID, rule)
}
