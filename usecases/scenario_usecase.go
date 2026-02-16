package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ScenarioUsecase struct {
	transactionFactory        executor_factory.TransactionFactory
	scenarioFetcher           scenarios.ScenarioFetcher
	validateScenarioAst       scenarios.ValidateScenarioAst
	executorFactory           executor_factory.ExecutorFactory
	enforceSecurity           security.EnforceSecurityScenario
	repository                repositories.ScenarioUsecaseRepository
	workflowRepository        workflowRepository
	iterationRepository       IterationUsecaseRepository
	screeningConfigRepository ScreeningConfigRepository
}

func (usecase *ScenarioUsecase) ListScenarios(ctx context.Context, organizationId uuid.UUID) ([]models.Scenario, error) {
	scenarios, err := usecase.repository.ListScenariosOfOrganization(ctx,
		usecase.executorFactory.NewExecutor(), organizationId)
	if err != nil {
		return nil, err
	}

	for _, scenario := range scenarios {
		if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
			return nil, err
		}
	}
	return scenarios, nil
}

func (usecase *ScenarioUsecase) GetScenario(ctx context.Context, scenarioId string) (models.Scenario, error) {
	scenario, err := usecase.repository.GetScenarioById(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
	if err != nil {
		return models.Scenario{}, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return models.Scenario{}, err
	}

	return scenario, nil
}

func (usecase *ScenarioUsecase) UpdateScenario(
	ctx context.Context,
	scenarioInput models.UpdateScenarioInput,
) (models.Scenario, error) {
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Scenario, error) {
			scenario, err := usecase.repository.GetScenarioById(ctx, tx, scenarioInput.Id)
			if err != nil {
				return models.Scenario{}, err
			}

			if err := usecase.enforceSecurity.UpdateScenario(scenario); err != nil {
				return models.Scenario{}, err
			}

			err = usecase.repository.UpdateScenario(ctx, tx, scenarioInput)
			if err != nil {
				return models.Scenario{}, err
			}
			scenario, err = usecase.repository.GetScenarioById(ctx, tx, scenario.Id)
			return scenario, errors.HandledWithMessage(err, "Error getting scenario after update")
		},
	)
}

func (usecase *ScenarioUsecase) ValidateScenarioAst(ctx context.Context,
	scenarioId string, astNode *ast.Node, expectedReturnType ...string,
) (validation models.AstValidation, err error) {
	scenario, err := usecase.scenarioFetcher.FetchScenario(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
	if err != nil {
		return validation, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return validation, err
	}

	validation = usecase.validateScenarioAst.Validate(ctx, scenario, astNode, expectedReturnType...)

	return validation, nil
}

func (usecase *ScenarioUsecase) CreateScenario(
	ctx context.Context,
	scenario models.CreateScenarioInput,
) (models.Scenario, error) {
	if err := usecase.enforceSecurity.CreateScenario(scenario.OrganizationId); err != nil {
		return models.Scenario{}, err
	}

	createdScenario, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Scenario, error) {
			newScenarioId := pure_utils.NewPrimaryKey(scenario.OrganizationId)
			if err := usecase.repository.CreateScenario(ctx, tx, scenario.OrganizationId, scenario, newScenarioId); err != nil {
				return models.Scenario{}, err
			}
			scenario, err := usecase.repository.GetScenarioById(ctx, tx, newScenarioId)
			return scenario, errors.HandledWithMessage(err, "Error getting scenario after update")
		},
	)
	if err != nil {
		return models.Scenario{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsScenarioCreated, map[string]interface{}{
		"scenario_id": createdScenario.Id,
	})
	return createdScenario, nil
}

func (usecase *ScenarioUsecase) ListLatestRules(ctx context.Context, scenarioId string) ([]models.ScenarioRuleLatestVersion, error) {
	scenario, err := usecase.repository.GetScenarioById(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
	if err != nil {
		return nil, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return nil, err
	}

	return usecase.repository.ListScenarioLatestRuleVersions(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
}

func (usecase *ScenarioUsecase) CopyScenario(
	ctx context.Context,
	scenarioId string,
	newName *string,
) (models.Scenario, error) {
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Scenario, error) {
			// Fetch the source scenario
			sourceScenario, err := usecase.repository.GetScenarioById(ctx, tx, scenarioId)
			if err != nil {
				return models.Scenario{}, err
			}

			if err := usecase.enforceSecurity.ReadScenario(sourceScenario); err != nil {
				return models.Scenario{}, err
			}

			if err := usecase.enforceSecurity.CreateScenario(sourceScenario.OrganizationId); err != nil {
				return models.Scenario{}, err
			}

			// Find the latest iteration (highest version, or draft if no published)
			scenarioIdUUID, err := uuid.Parse(scenarioId)
			if err != nil {
				return models.Scenario{}, errors.Wrap(err, "invalid scenario id")
			}

			iterations, err := usecase.iterationRepository.ListScenarioIterations(
				ctx, tx, sourceScenario.OrganizationId,
				models.GetScenarioIterationFilters{ScenarioId: scenarioIdUUID},
			)
			if err != nil {
				return models.Scenario{}, errors.Wrap(err, "failed to list scenario iterations")
			}

			if len(iterations) == 0 {
				return models.Scenario{}, errors.Wrap(models.NotFoundError, "no iterations found for scenario")
			}

			// Find the latest iteration: prefer the highest version, or draft if no published
			var sourceIteration *models.ScenarioIteration
			var highestVersion int
			for i := range iterations {
				if iterations[i].Version != nil {
					if *iterations[i].Version > highestVersion {
						highestVersion = *iterations[i].Version
						sourceIteration = &iterations[i]
					}
				}
			}
			// If no published version found, use draft
			if sourceIteration == nil {
				for i := range iterations {
					if iterations[i].Version == nil {
						sourceIteration = &iterations[i]
						break
					}
				}
			}

			if sourceIteration == nil {
				return models.Scenario{}, errors.Wrap(models.NotFoundError, "no suitable iteration found")
			}

			// Load screening configs for the source iteration
			screeningConfigs, err := usecase.screeningConfigRepository.ListScreeningConfigs(ctx, tx, sourceIteration.Id, false)
			if err != nil {
				return models.Scenario{}, errors.Wrap(err, "failed to list screening configs")
			}

			// Determine the new scenario name
			scenarioName := "Copy of " + sourceScenario.Name
			if newName != nil && *newName != "" {
				scenarioName = *newName
			}

			// Create the new scenario
			newScenarioId := pure_utils.NewPrimaryKey(sourceScenario.OrganizationId)
			createScenarioInput := models.CreateScenarioInput{
				Description:       sourceScenario.Description,
				Name:              scenarioName,
				TriggerObjectType: sourceScenario.TriggerObjectType,
				OrganizationId:    sourceScenario.OrganizationId,
			}

			if err := usecase.repository.CreateScenario(ctx, tx, sourceScenario.OrganizationId, createScenarioInput, newScenarioId); err != nil {
				return models.Scenario{}, errors.Wrap(err, "failed to create new scenario")
			}

			// Create a draft iteration on the new scenario with new stable IDs
			createIterationInput := models.CreateScenarioIterationInput{
				ScenarioId: newScenarioId,
				Body: models.CreateScenarioIterationBody{
					ScoreReviewThreshold:          sourceIteration.ScoreReviewThreshold,
					ScoreBlockAndReviewThreshold:  sourceIteration.ScoreBlockAndReviewThreshold,
					ScoreDeclineThreshold:         sourceIteration.ScoreDeclineThreshold,
					Schedule:                      sourceIteration.Schedule,
					Rules:                         make([]models.CreateRuleInput, len(sourceIteration.Rules)),
					TriggerConditionAstExpression: sourceIteration.TriggerConditionAstExpression,
				},
			}

			// Copy rules with NEW stable IDs (different from CreateDraftFromScenarioIteration)
			// SnoozeGroupId and StableRuleId are NOT copied - new ones will be generated
			// so the copied rules are independent from the original
			for i, rule := range sourceIteration.Rules {
				createIterationInput.Body.Rules[i] = models.CreateRuleInput{
					DisplayOrder:         rule.DisplayOrder,
					Name:                 rule.Name,
					Description:          rule.Description,
					FormulaAstExpression: rule.FormulaAstExpression,
					ScoreModifier:        rule.ScoreModifier,
					RuleGroup:            rule.RuleGroup,
				}
			}

			newIteration, err := usecase.iterationRepository.CreateScenarioIterationAndRules(
				ctx, tx, sourceScenario.OrganizationId, createIterationInput)
			if err != nil {
				return models.Scenario{}, errors.Wrap(err, "failed to create new iteration")
			}

			// Copy screening configs with NEW stable IDs
			for _, scc := range screeningConfigs {
				newScreeningConfig := models.UpdateScreeningConfigInput{
					// StableId is NOT copied - a new one will be generated
					Name:                     &scc.Name,
					Description:              &scc.Description,
					RuleGroup:                scc.RuleGroup,
					EntityType:               &scc.EntityType,
					Datasets:                 scc.Datasets,
					Threshold:                scc.Threshold,
					TriggerRule:              scc.TriggerRule,
					CounterpartyIdExpression: scc.CounterpartyIdExpression,
					Query:                    scc.Query,
					ForcedOutcome:            &scc.ForcedOutcome,
					Preprocessing:            &scc.Preprocessing,
					ConfigVersion:            scc.ConfigVersion,
				}
				if _, err := usecase.screeningConfigRepository.CreateScreeningConfig(
					ctx, tx, newIteration.Id, newScreeningConfig); err != nil {
					return models.Scenario{}, errors.Wrap(err, "failed to copy screening config")
				}
			}

			// Copy workflow rules with their conditions and actions
			workflows, err := usecase.workflowRepository.ListWorkflowsForScenario(ctx, tx, scenarioIdUUID)
			if err != nil {
				return models.Scenario{}, errors.Wrap(err, "failed to list workflows")
			}

			newScenarioIdUUID, err := uuid.Parse(newScenarioId)
			if err != nil {
				return models.Scenario{}, errors.Wrap(err, "invalid new scenario id")
			}

			for _, workflow := range workflows {
				// Create new workflow rule
				newWorkflowRule := models.WorkflowRule{
					ScenarioId:  newScenarioIdUUID,
					Name:        workflow.WorkflowRule.Name,
					Priority:    workflow.WorkflowRule.Priority,
					Fallthrough: workflow.WorkflowRule.Fallthrough,
				}
				createdRule, err := usecase.workflowRepository.InsertWorkflowRule(ctx, tx, newWorkflowRule)
				if err != nil {
					return models.Scenario{}, errors.Wrap(err, "failed to copy workflow rule")
				}

				// Copy conditions
				for _, cond := range workflow.Conditions {
					newCondition := models.WorkflowCondition{
						RuleId:   createdRule.Id,
						Function: cond.Function,
						Params:   cond.Params,
					}
					if _, err := usecase.workflowRepository.InsertWorkflowCondition(ctx, tx, newCondition); err != nil {
						return models.Scenario{}, errors.Wrap(err, "failed to copy workflow condition")
					}
				}

				// Copy actions
				for _, action := range workflow.Actions {
					newAction := models.WorkflowAction{
						RuleId: createdRule.Id,
						Action: action.Action,
						Params: action.Params,
					}
					if _, err := usecase.workflowRepository.InsertWorkflowAction(ctx, tx, newAction); err != nil {
						return models.Scenario{}, errors.Wrap(err, "failed to copy workflow action")
					}
				}
			}

			// Return the newly created scenario
			newScenario, err := usecase.repository.GetScenarioById(ctx, tx, newScenarioId)
			if err != nil {
				return models.Scenario{}, errors.Wrap(err, "failed to get new scenario")
			}

			tracking.TrackEvent(ctx, models.AnalyticsScenarioCreated, map[string]interface{}{
				"scenario_id":    newScenario.Id,
				"copied_from_id": scenarioId,
			})

			return newScenario, nil
		},
	)
}
