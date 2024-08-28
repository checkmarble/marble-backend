package usecases

import (
	"context"
	"fmt"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
)

type IterationUsecaseRepository interface {
	GetScenarioIteration(
		ctx context.Context,
		exec repositories.Executor,
		scenarioIterationId string,
	) (models.ScenarioIteration, error)
	ListScenarioIterations(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		filters models.GetScenarioIterationFilters,
	) ([]models.ScenarioIteration, error)

	CreateScenarioIterationAndRules(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		scenarioIteration models.CreateScenarioIterationInput,
	) (models.ScenarioIteration, error)
	UpdateScenarioIteration(
		ctx context.Context,
		exec repositories.Executor,
		scenarioIteration models.UpdateScenarioIterationInput,
	) (models.ScenarioIteration, error)
	UpdateScenarioIterationVersion(
		ctx context.Context,
		exec repositories.Executor,
		scenarioIterationId string,
		newVersion int,
	) error
	DeleteScenarioIteration(
		ctx context.Context,
		exec repositories.Executor,
		scenarioIterationId string,
	) error
}

type ScenarioIterationUsecase struct {
	repository                IterationUsecaseRepository
	enforceSecurity           security.EnforceSecurityScenario
	scenarioFetcher           scenarios.ScenarioFetcher
	validateScenarioIteration scenarios.ValidateScenarioIteration
	executorFactory           executor_factory.ExecutorFactory
	transactionFactory        executor_factory.TransactionFactory
}

func (usecase *ScenarioIterationUsecase) ListScenarioIterations(
	ctx context.Context,
	organizationId string,
	filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIteration, error) {
	scenarioIterations, err := usecase.repository.ListScenarioIterations(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, filters)
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

func (usecase *ScenarioIterationUsecase) GetScenarioIteration(ctx context.Context,
	scenarioIterationId string,
) (models.ScenarioIteration, error) {
	si, err := usecase.repository.GetScenarioIteration(ctx,
		usecase.executorFactory.NewExecutor(), scenarioIterationId)
	if err != nil {
		return models.ScenarioIteration{}, err
	}
	if err := usecase.enforceSecurity.ReadScenarioIteration(si); err != nil {
		return models.ScenarioIteration{}, err
	}
	return si, nil
}

func (usecase *ScenarioIterationUsecase) CreateScenarioIteration(ctx context.Context,
	organizationId string, scenarioIteration models.CreateScenarioIterationInput,
) (models.ScenarioIteration, error) {
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

	if body == nil {
		body = &models.CreateScenarioIterationBody{}
		scenarioIteration.Body = body
	}

	if body.ScoreReviewThreshold == nil {
		defaultReviewThreshold := 0
		body.ScoreReviewThreshold = &defaultReviewThreshold
	}

	if body.ScoreRejectThreshold == nil {
		defaultRejectThreshold := 10
		body.ScoreRejectThreshold = &defaultRejectThreshold
	}

	si, err := usecase.repository.CreateScenarioIterationAndRules(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, scenarioIteration)
	if err != nil {
		return models.ScenarioIteration{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsScenarioIterationCreated, map[string]interface{}{
		"scenario_iteration_id": si.Id,
	})

	return si, nil
}

func (usecase *ScenarioIterationUsecase) UpdateScenarioIteration(ctx context.Context,
	organizationId string, scenarioIteration models.UpdateScenarioIterationInput,
) (iteration models.ScenarioIteration, err error) {
	exec := usecase.executorFactory.NewExecutor()
	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, exec, scenarioIteration.Id)
	if err != nil {
		return iteration, err
	}
	if err := usecase.enforceSecurity.UpdateScenario(scenarioAndIteration.Scenario); err != nil {
		return iteration, err
	}
	body := scenarioIteration.Body
	if body.Schedule != nil && *body.Schedule != "" {
		gron := gronx.New()
		ok := gron.IsValid(*body.Schedule)
		if !ok {
			return iteration, fmt.Errorf("invalid schedule: %w", models.BadParameterError)
		}
	}
	if scenarioAndIteration.Iteration.Version != nil {
		return iteration, errors.Wrap(
			models.ErrScenarioIterationNotDraft,
			fmt.Sprintf("iteration %s is not a draft", scenarioAndIteration.Iteration.Id),
		)
	}
	return usecase.repository.UpdateScenarioIteration(ctx, exec, scenarioIteration)
}

func (usecase *ScenarioIterationUsecase) CreateDraftFromScenarioIteration(ctx context.Context,
	organizationId string, scenarioIterationId string,
) (models.ScenarioIteration, error) {
	newScenarioIteration, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.ScenarioIteration, error) {
			if err := usecase.enforceSecurity.CreateScenario(organizationId); err != nil {
				return models.ScenarioIteration{}, err
			}
			si, err := usecase.repository.GetScenarioIteration(ctx, tx, scenarioIterationId)
			if err != nil {
				return models.ScenarioIteration{}, err
			}
			iterations, err := usecase.repository.ListScenarioIterations(
				ctx,
				tx,
				organizationId,
				models.GetScenarioIterationFilters{ScenarioId: &si.ScenarioId},
			)
			if err != nil {
				return models.ScenarioIteration{}, err
			}
			for _, iteration := range iterations {
				if iteration.Version == nil {
					err = usecase.repository.DeleteScenarioIteration(ctx, tx, iteration.Id)
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
					RuleGroup:            rule.RuleGroup,
					SnoozeGroupId:        rule.SnoozeGroupId,
				}
			}
			return usecase.repository.CreateScenarioIterationAndRules(ctx, tx, organizationId, createScenarioIterationInput)
		})
	if err != nil {
		return models.ScenarioIteration{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsScenarioIterationCreated, map[string]interface{}{
		"scenario_iteration_id": newScenarioIteration.Id,
	})

	return newScenarioIteration, nil
}

// Return a validation by running the scenario using fake data
// If `triggerOrRuleToReplace` is provided, it is used during the validation.
// If `replaceRuleId` is provided, the corresponding rule is replaced.
// if `replaceRuleId` is nil, the trigger is replaced.
func (usecase *ScenarioIterationUsecase) ValidateScenarioIteration(ctx context.Context,
	iterationId string, triggerOrRuleToReplace *ast.Node, ruleIdToReplace *string,
) (validation models.ScenarioValidation, err error) {
	scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx,
		usecase.executorFactory.NewExecutor(), iterationId)
	if err != nil {
		return validation, err
	}

	if err := usecase.enforceSecurity.ReadScenarioIteration(scenarioAndIteration.Iteration); err != nil {
		return validation, err
	}

	scenarioAndIteration, err = replaceTriggerOrRule(scenarioAndIteration,
		triggerOrRuleToReplace, ruleIdToReplace)
	if err != nil {
		return validation, err
	}
	validation, err = usecase.validateScenarioIteration.Validate(ctx, scenarioAndIteration), nil
	return validation, err
}

func (usecase *ScenarioIterationUsecase) CommitScenarioIterationVersion(
	ctx context.Context,
	iterationId string,
) (iteration models.ScenarioIteration, err error) {
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.ScenarioIteration, error) {
			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, iterationId)
			if err != nil {
				return iteration, err
			}
			if err := usecase.enforceSecurity.UpdateScenario(scenarioAndIteration.Scenario); err != nil {
				return iteration, err
			}
			if scenarioAndIteration.Iteration.Version != nil {
				return iteration, errors.Wrap(
					models.ErrScenarioIterationNotDraft,
					fmt.Sprintf("input scenario iteration %s is a draft in CommitScenarioIterationVersion", iterationId),
				)
			}
			validation := usecase.validateScenarioIteration.Validate(ctx, scenarioAndIteration)
			if err := scenarios.ScenarioValidationToError(validation); err != nil {
				return iteration, errors.Wrap(models.BadParameterError,
					fmt.Sprintf("Scenario iteration %s is not valid", iterationId),
				)
			}
			version, err := usecase.getScenarioVersion(
				ctx,
				tx,
				scenarioAndIteration.Scenario.OrganizationId,
				scenarioAndIteration.Scenario.Id,
			)
			if err != nil {
				return iteration, err
			}
			if err = usecase.repository.UpdateScenarioIterationVersion(ctx, tx, iterationId, version); err != nil {
				return iteration, err
			}
			return usecase.repository.GetScenarioIteration(ctx, tx, iterationId)
		},
	)
}

func replaceTriggerOrRule(scenarioAndIteration models.ScenarioAndIteration,
	triggerOrRuleToReplace *ast.Node, ruleIdToReplace *string,
) (models.ScenarioAndIteration, error) {
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

func (usecase *ScenarioIterationUsecase) getScenarioVersion(
	ctx context.Context,
	exec repositories.Executor,
	organizationId, scenarioId string,
) (int, error) {
	scenarioIterations, err := usecase.repository.ListScenarioIterations(
		ctx,
		exec,
		organizationId,
		models.GetScenarioIterationFilters{ScenarioId: &scenarioId})
	if err != nil {
		return 0, err
	}

	var latestVersion int
	for _, scenarioIteration := range scenarioIterations {
		if scenarioIteration.Version != nil && *scenarioIteration.Version > latestVersion {
			latestVersion = *scenarioIteration.Version
		}
	}
	newVersion := latestVersion + 1

	return newVersion, nil
}
