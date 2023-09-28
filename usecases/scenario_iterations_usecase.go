package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"

	"github.com/adhocore/gronx"
)

type IterationUsecaseRepository interface {
	GetScenarioIteration(tx repositories.Transaction, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)
	ListScenarioIterations(tx repositories.Transaction, organizationId string, filters models.GetScenarioIterationFilters) (
		[]models.ScenarioIteration, error,
	)

	CreateScenarioIterationAndRules(tx repositories.Transaction, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error)
	UpdateScenarioIteration(tx repositories.Transaction, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error)
	UpdateScenarioIterationVersion(tx repositories.Transaction, scenarioIterationId string, newVersion int) error
	DeleteScenarioIteration(tx repositories.Transaction, scenarioIterationId string) error
}

type ScenarioIterationUsecase struct {
	repository                IterationUsecaseRepository
	organizationIdOfContext   func() (string, error)
	enforceSecurity           security.EnforceSecurityScenario
	scenarioFetcher           scenarios.ScenarioFetcher
	validateScenarioIteration scenarios.ValidateScenarioIteration
}

func (usecase *ScenarioIterationUsecase) ListScenarioIterations(filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return nil, err
	}
	scenarioIterations, err := usecase.repository.ListScenarioIterations(nil, organizationId, filters)
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
	si, err := usecase.repository.GetScenarioIteration(nil, scenarioIterationId)
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
	return usecase.repository.CreateScenarioIterationAndRules(nil, organizationId, scenarioIteration)
}

func (usecase *ScenarioIterationUsecase) UpdateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.UpdateScenarioIterationInput) (iteration models.ScenarioIteration, err error) {
	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, scenarioIteration.Id)
	if err != nil {
		return iteration, err
	}
	if err := usecase.enforceSecurity.UpdateScenario(scenarioAndIteration.Scenario); err != nil {
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
	if scenarioAndIteration.Iteration.Version != nil {
		return iteration, fmt.Errorf("iteration is not a draft: %w", models.ErrScenarioIterationNotDraft)
	}
	return usecase.repository.UpdateScenarioIteration(nil, scenarioIteration)
}

func (usecase *ScenarioIterationUsecase) CreateDraftFromScenarioIteration(ctx context.Context, organizationId string, scenarioIterationId string) (models.ScenarioIteration, error) {
	if err := usecase.enforceSecurity.CreateScenario(organizationId); err != nil {
		return models.ScenarioIteration{}, err
	}
	si, err := usecase.repository.GetScenarioIteration(nil, scenarioIterationId)
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	iterations, err := usecase.repository.ListScenarioIterations(nil, organizationId, models.GetScenarioIterationFilters{
		ScenarioId: &si.ScenarioId,
	})
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	for _, iteration := range iterations {
		if iteration.Version == nil {
			err = usecase.repository.DeleteScenarioIteration(nil, iteration.Id)
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
	return usecase.repository.CreateScenarioIterationAndRules(nil, organizationId, createScenarioIterationInput)
}

// Return a validation by running the scenario using fake data
// If `triggerOrRuleToReplace` is provided, it is used during the validation.
// If `replaceRuleId` is provided, the corresponding rule is replaced.
// if `replaceRuleId` is nil, the trigger is replaced.
func (usecase *ScenarioIterationUsecase) ValidateScenarioIteration(iterationId string, triggerOrRuleToReplace *ast.Node, ruleIdToReplace *string) (validation models.ScenarioValidation, err error) {

	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(nil, iterationId)
	if err != nil {
		return validation, err
	}

	if err := usecase.enforceSecurity.CreateScenario(scenarioAndIteration.Scenario.OrganizationId); err != nil {
		return validation, err
	}

	scenarioAndIteration, err = replaceTriggerOrRule(scenarioAndIteration, triggerOrRuleToReplace, ruleIdToReplace)
	if err != nil {
		return validation, err
	}
	return usecase.validateScenarioIteration.Validate(scenarioAndIteration), nil
}

func replaceTriggerOrRule(scenarioAndIteration scenarios.ScenarioAndIteration, triggerOrRuleToReplace *ast.Node, ruleIdToReplace *string) (scenarios.ScenarioAndIteration, error) {

	if triggerOrRuleToReplace != nil {
		if ruleIdToReplace != nil {
			var found bool
			for index, rule := range scenarioAndIteration.Iteration.Rules {
				if rule.Id == *ruleIdToReplace {
					scenarioAndIteration.Iteration.Rules[index].FormulaAstExpression = triggerOrRuleToReplace
					found = true
					break
				}
			}
			if !found {
				return scenarioAndIteration, fmt.Errorf("rule not found: %w", models.NotFoundError)
			}
		} else {
			scenarioAndIteration.Iteration.TriggerConditionAstExpression = triggerOrRuleToReplace
		}
	}

	return scenarioAndIteration, nil
}
