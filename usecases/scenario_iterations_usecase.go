package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/scenarios"
	"marble/marble-backend/usecases/security"

	"github.com/adhocore/gronx"
)

type ScenarioIterationUsecase struct {
	organizationIdOfContext                 func() (string, error)
	scenarioIterationsReadRepository        repositories.ScenarioIterationReadRepository
	scenarioIterationsWriteRepository       repositories.ScenarioIterationWriteRepository
	enforceSecurity                         security.EnforceSecurityScenario
	scenarioFetcher                         scenarios.ScenarioFetcher
	validateScenarioIteration               scenarios.ValidateScenarioIteration
}

func (usecase *ScenarioIterationUsecase) ListScenarioIterations(filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return nil, err
	}
	scenarioIterations, err := usecase.scenarioIterationsReadRepository.ListScenarioIterations(nil, organizationId, filters)
	if err != nil {
		return nil, err
	}
	for _, si := range scenarioIterations {
		if err := usecase.enforceSecurity.ReadScenarioIteration(si); err != nil {
			return nil, err
		}
	}
	return scenarioIterations, nil
}

func (usecase *ScenarioIterationUsecase) GetScenarioIteration(scenarioIterationId string) (models.ScenarioIteration, error) {
	si, err := usecase.scenarioIterationsReadRepository.GetScenarioIteration(nil, scenarioIterationId)
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	if err := usecase.enforceSecurity.ReadScenarioIteration(si); err != nil {
		return models.ScenarioIteration{}, err
	}
	return si, nil
}

func (usecase *ScenarioIterationUsecase) CreateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error) {
	if err := usecase.enforceSecurity.CreateScenario(organizationId); err != nil {
		return models.ScenarioIteration{}, err
	}
	body := scenarioIteration.Body
	if body != nil && body.Schedule != "" {
		gron := gronx.New()
		ok := gron.IsValid(body.Schedule)
		if !ok {
			return models.ScenarioIteration{}, fmt.Errorf("invalid schedule: %w", models.BadParameterError)
		}
	}
	return usecase.scenarioIterationsWriteRepository.CreateScenarioIteration(nil, organizationId, scenarioIteration)
}

func (usecase *ScenarioIterationUsecase) UpdateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.UpdateScenarioIterationInput) (iteration models.ScenarioIteration, err error) {
	if err := usecase.enforceSecurity.CreateScenario(organizationId); err != nil {
		return iteration, err
	}
	body := scenarioIteration.Body
	if body != nil && body.Schedule != nil && *body.Schedule != "" {
		gron := gronx.New()
		ok := gron.IsValid(*body.Schedule)
		if !ok {
			return iteration, fmt.Errorf("invalid schedule: %w", models.BadParameterError)
		}
	}
	si, err := usecase.scenarioIterationsReadRepository.GetScenarioIteration(nil, scenarioIteration.Id)
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	if err := usecase.enforceSecurity.UpdateScenarioIteration(organizationId, si); err != nil {
		return models.ScenarioIteration{}, err
	}
	if _, err = usecase.scenarioIterationsWriteRepository.UpdateScenarioIteration(nil, organizationId, scenarioIteration); err != nil {
		return iteration, err
	}

	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, scenarioIteration.Id)
	if err != nil {
		return iteration, err
	}

	return scenarioAndIteration.Iteration, err
}

func (usecase *ScenarioIterationUsecase) CreateDraftFromScenarioIteration(ctx context.Context, organizationId string, scenarioIterationId string) (models.ScenarioIteration, error) {
	if err := usecase.enforceSecurity.CreateScenario(organizationId); err != nil {
		return models.ScenarioIteration{}, err
	}
	si, err := usecase.scenarioIterationsReadRepository.GetScenarioIteration(nil, scenarioIterationId)
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	iterations, err := usecase.scenarioIterationsReadRepository.ListScenarioIterations(nil, organizationId, models.GetScenarioIterationFilters{
		ScenarioId: &si.ScenarioId,
	})
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	for _, iteration := range iterations {
		if iteration.Version == nil {
			err = usecase.scenarioIterationsWriteRepository.DeleteScenarioIteration(nil, iteration.Id)
			if err != nil {
				return models.ScenarioIteration{}, err
			}
		}
	}
	createScenarioIterationInput := models.CreateScenarioIterationInput{
		ScenarioId: si.ScenarioId,
	}
	createScenarioIterationInput.Body = &models.CreateScenarioIterationBody{
		ScoreReviewThreshold:          si.ScoreReviewThreshold,
		ScoreRejectThreshold:          si.ScoreRejectThreshold,
		BatchTriggerSQL:               si.BatchTriggerSQL,
		Schedule:                      si.Schedule,
		Rules:                         make([]models.CreateRuleInput, len(si.Rules)),
		TriggerConditionAstExpression: si.TriggerConditionAstExpression,
	}

	for i, rule := range si.Rules {
		createScenarioIterationInput.Body.Rules[i] = models.CreateRuleInput{
			DisplayOrder:         rule.DisplayOrder,
			Name:                 rule.Name,
			Description:          rule.Description,
			FormulaAstExpression: rule.FormulaAstExpression,
			ScoreModifier:        rule.ScoreModifier,
		}
	}
	return usecase.scenarioIterationsWriteRepository.CreateScenarioIteration(nil, organizationId, createScenarioIterationInput)
}

func (usecase *ScenarioIterationUsecase) ValidateScenarioIteration(iterationId string) (validation models.ScenarioValidation, err error) {

	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, iterationId)
	if err != nil {
		return validation, err
	}

	if err := usecase.enforceSecurity.CreateScenario(scenarioAndIteration.Scenario.OrganizationId); err != nil {
		return validation, err
	}

	return usecase.validateScenarioIteration.Validate(scenarioAndIteration), nil
}
