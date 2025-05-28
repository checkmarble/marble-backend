package usecases

import (
	"context"
	"fmt"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
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

	UpdateRule(ctx context.Context, exec repositories.Executor, rule models.UpdateRuleInput) error
}

type ScenarioIterationUsecase struct {
	repository                    IterationUsecaseRepository
	sanctionCheckConfigRepository SanctionCheckConfigRepository
	enforceSecurity               security.EnforceSecurityScenario
	scenarioFetcher               scenarios.ScenarioFetcher
	validateScenarioIteration     scenarios.ValidateScenarioIteration
	executorFactory               executor_factory.ExecutorFactory
	transactionFactory            executor_factory.TransactionFactory
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

	scc, err := usecase.sanctionCheckConfigRepository.ListSanctionCheckConfigs(ctx,
		usecase.executorFactory.NewExecutor(), si.Id)
	if err != nil {
		return models.ScenarioIteration{}, errors.Wrap(err,
			"could not retrieve sanction check config while getting scenario iteration")
	}
	si.SanctionCheckConfigs = scc

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
		defaultReviewThreshold := 1
		body.ScoreReviewThreshold = &defaultReviewThreshold
	}

	defaultDeclineThreshold := 10
	if body.ScoreBlockAndReviewThreshold == nil {
		// the block and review outcome cannot be reached with the default scenario iteration
		body.ScoreBlockAndReviewThreshold = &defaultDeclineThreshold
	}

	if body.ScoreDeclineThreshold == nil {
		body.ScoreDeclineThreshold = &defaultDeclineThreshold
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
	updatedScenarioIteration, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.ScenarioIteration, error) {
			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, scenarioIteration.Id)
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

			return usecase.repository.UpdateScenarioIteration(ctx, tx, scenarioIteration)
		})
	if err != nil {
		return models.ScenarioIteration{}, err
	}

	return updatedScenarioIteration, nil
}

func (usecase *ScenarioIterationUsecase) CreateDraftFromScenarioIteration(
	ctx context.Context,
	organizationId string,
	scenarioIterationId string,
) (models.ScenarioIteration, error) {
	if err := usecase.enforceSecurity.CreateScenario(organizationId); err != nil {
		return models.ScenarioIteration{}, err
	}

	newScenarioIteration, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.ScenarioIteration, error) {
			si, err := usecase.repository.GetScenarioIteration(ctx, tx, scenarioIterationId)
			if err != nil {
				return models.ScenarioIteration{}, err
			}

			sanctionCheckConfigs, err := usecase.sanctionCheckConfigRepository.ListSanctionCheckConfigs(ctx, tx, si.Id)
			if err != nil {
				return models.ScenarioIteration{}, errors.Wrap(err,
					"could not retrieve sanction check config while creating draft")
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
				ScoreBlockAndReviewThreshold:  si.ScoreBlockAndReviewThreshold,
				ScoreDeclineThreshold:         si.ScoreDeclineThreshold,
				Schedule:                      si.Schedule,
				Rules:                         make([]models.CreateRuleInput, len(si.Rules)),
				TriggerConditionAstExpression: si.TriggerConditionAstExpression,
			}

			stableRuleGroupsToUpdate := make([]models.UpdateRuleInput, 0, len(si.Rules))
			for i, rule := range si.Rules {
				createScenarioIterationInput.Body.Rules[i] = models.CreateRuleInput{
					DisplayOrder:         rule.DisplayOrder,
					Name:                 rule.Name,
					Description:          rule.Description,
					FormulaAstExpression: rule.FormulaAstExpression,
					ScoreModifier:        rule.ScoreModifier,
					RuleGroup:            rule.RuleGroup,
					SnoozeGroupId:        rule.SnoozeGroupId,
					StableRuleId:         rule.StableRuleId,
				}

				// old rules may not have a stableGroupId. If so, when creating a new draft, we create new stable rule ids
				// for the new draft rules, and backfill them on the version from which the draft is created.
				// TODO: later, when we are confident no more iterations are being created from old versions, we could backfill
				// random stable group ids on old rules and make the field not null.
				if rule.StableRuleId == nil {
					newId := uuid.NewString()
					createScenarioIterationInput.Body.Rules[i].StableRuleId = &newId
					stableRuleGroupsToUpdate = append(stableRuleGroupsToUpdate, models.UpdateRuleInput{
						Id:           rule.Id,
						StableRuleId: &newId,
					})
				}
			}
			for _, updateOldRule := range stableRuleGroupsToUpdate {
				err := usecase.repository.UpdateRule(ctx, tx, updateOldRule)
				if err != nil {
					return models.ScenarioIteration{}, err
				}
			}

			newScenarioIteration, err := usecase.repository.CreateScenarioIterationAndRules(
				ctx, tx, organizationId, createScenarioIterationInput)

			if len(sanctionCheckConfigs) > 0 {
				newSanctionCheckConfigs := pure_utils.Map(sanctionCheckConfigs, func(
					scc models.SanctionCheckConfig,
				) models.UpdateSanctionCheckConfigInput {
					return models.UpdateSanctionCheckConfigInput{
						StableId:                 &scc.StableId,
						Name:                     &scc.Name,
						Description:              &scc.Description,
						RuleGroup:                scc.RuleGroup,
						Datasets:                 scc.Datasets,
						TriggerRule:              scc.TriggerRule,
						CounterpartyIdExpression: scc.CounterpartyIdExpression,
						Query:                    scc.Query,
						ForcedOutcome:            &scc.ForcedOutcome,
					}
				})

				for _, scc := range newSanctionCheckConfigs {
					if _, err := usecase.sanctionCheckConfigRepository.CreateSanctionCheckConfig(
						ctx, tx, newScenarioIteration.Id, scc); err != nil {
						return models.ScenarioIteration{}, errors.Wrap(err,
							"could not duplicate sanction check config for new iteration")
					}
				}
			}

			return newScenarioIteration, err
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
		func(tx repositories.Transaction) (models.ScenarioIteration, error) {
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
