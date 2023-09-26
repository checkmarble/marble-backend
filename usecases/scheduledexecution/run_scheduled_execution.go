package scheduledexecution

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/adhocore/gronx"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
)

type RunScheduledExecution struct {
	ScenarioReadRepository          repositories.ScenarioReadRepository
	TransactionFactory              transaction.TransactionFactory
	ScheduledExecutionRepository    repositories.ScheduledExecutionRepository
	ExportScheduleExecution         ExportScheduleExecution
	ScenarioPublicationsRepository  repositories.ScenarioPublicationRepository
	DataModelRepository             repositories.DataModelRepository
	OrgTransactionFactory           transaction.Factory
	IngestedDataReadRepository      repositories.IngestedDataReadRepository
	ScenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	EvaluateRuleAstExpression       ast_eval.EvaluateRuleAstExpression
	DecisionRepository              repositories.DecisionRepository
}

func (usecase *RunScheduledExecution) ExecuteScheduledScenarioIfDue(ctx context.Context, organizationId string, scenarioId string) (err error) {
	logger := utils.LoggerFromContext(ctx)
	// This is called by a cron job, for all scheduled scenarios. It is crucial that a panic on one scenario does not break all the others.
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorContext(ctx, "recovered from panic during scheduled scenario execution. Stacktrace from panic: ")
			logger.ErrorContext(ctx, string(debug.Stack()))
			err = fmt.Errorf("recovered from panic during scheduled scenario execution")
		}
	}()

	scenario, err := usecase.ScenarioReadRepository.GetScenarioById(nil, scenarioId)
	if err != nil {
		return err
	}

	publishedVersion, err := usecase.getPublishedScenarioIteration(scenario)
	if err != nil {
		return err
	}
	if publishedVersion == nil {
		logger.DebugContext(ctx, fmt.Sprintf("scenario %s has no published version", scenarioId))
		return nil
	}

	isDue, err := usecase.scenarioIsDue(ctx, *publishedVersion, scenario)
	if err != nil {
		return err
	}

	if isDue {
		logger := utils.LoggerFromContext(ctx)
		logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s is due", publishedVersion.Id))

		scheduledExecution, err := transaction.TransactionReturnValue(
			usecase.TransactionFactory,
			models.DATABASE_MARBLE_SCHEMA,
			func(tx repositories.Transaction) (models.ScheduledExecution, error) {
				scheduledExecutionId := utils.NewPrimaryKey(organizationId)
				if err := usecase.ScheduledExecutionRepository.CreateScheduledExecution(tx, models.CreateScheduledExecutionInput{
					OrganizationId:      organizationId,
					ScenarioId:          scenarioId,
					ScenarioIterationId: publishedVersion.Id,
				}, scheduledExecutionId); err != nil {
					return models.ScheduledExecution{}, err
				}

				if err := usecase.ScheduledExecutionRepository.UpdateScheduledExecution(tx, models.UpdateScheduledExecutionInput{
					Id:     scheduledExecutionId,
					Status: utils.PtrTo(models.ScheduledExecutionFailure, nil),
				}); err != nil {
					return models.ScheduledExecution{}, err
				}

				// Actually execute the scheduled scenario
				numberOfCreatedDecisions, err := usecase.executeScheduledScenario(ctx, scheduledExecutionId, scenario)

				if err != nil {
					return models.ScheduledExecution{}, err
				}

				if err := usecase.ScheduledExecutionRepository.UpdateScheduledExecution(tx, models.UpdateScheduledExecutionInput{
					Id:                       scheduledExecutionId,
					Status:                   utils.PtrTo(models.ScheduledExecutionSuccess, nil),
					NumberOfCreatedDecisions: &numberOfCreatedDecisions,
				}); err != nil {
					return models.ScheduledExecution{}, err
				}
				// Mark the scheduled scenario as sucess
				logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s executed successfully", publishedVersion.Id))

				return usecase.ScheduledExecutionRepository.GetScheduledExecution(tx, scheduledExecutionId)
			},
		)

		if err != nil {
			return err
		}

		// export decisions
		return usecase.ExportScheduleExecution.ExportScheduledExecutionToS3(scenario, scheduledExecution)
	}

	return nil
}

func (usecase *RunScheduledExecution) scenarioIsDue(ctx context.Context, publishedVersion models.PublishedScenarioIteration, scenario models.Scenario) (bool, error) {
	logger := utils.LoggerFromContext(ctx)
	if publishedVersion.Body.Schedule == "" {
		logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s has no schedule", publishedVersion.Id))
		return false, nil
	}
	gron := gronx.New()
	ok := gron.IsValid(publishedVersion.Body.Schedule)
	if !ok {
		return false, fmt.Errorf("invalid schedule: %w", models.BadParameterError)
	}

	previousExecutions, err := usecase.ScheduledExecutionRepository.ListScheduledExecutionsOfScenario(nil, scenario.Id)
	if err != nil {
		return false, fmt.Errorf("error listing scheduled executions: %w", err)
	}

	publications, err := usecase.ScenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(nil, scenario.OrganizationId, models.ListScenarioPublicationsFilters{ScenarioId: &scenario.Id})
	if err != nil {
		return false, err
	}

	tz, _ := time.LoadLocation("Europe/Paris")
	return executionIsDueNow(publishedVersion.Body.Schedule, previousExecutions, publications, tz)
}

func executionIsDueNow(schedule string, previousExecutions []models.ScheduledExecution, publications []models.ScenarioPublication, tz *time.Location) (bool, error) {
	var referenceTime time.Time
	if len(previousExecutions) > 0 {
		referenceTime = previousExecutions[0].StartedAt.In(tz)
	} else {
		// if there is no previous execution, consider the last iteration publication time to be the last execution time
		referenceTime = publications[0].CreatedAt.In(tz)
	}

	nextTick, err := gronx.NextTickAfter(schedule, referenceTime, false)
	if err != nil {
		return true, err
	}
	if nextTick.After(time.Now()) {
		return false, nil
	}
	return true, nil
}

func (usecase *RunScheduledExecution) executeScheduledScenario(ctx context.Context, scheduledExecutionId string, scenario models.Scenario) (int, error) {
	dataModel, err := usecase.DataModelRepository.GetDataModel(nil, scenario.OrganizationId)
	if err != nil {
		return 0, err
	}
	tables := dataModel.Tables
	table, ok := tables[models.TableName(scenario.TriggerObjectType)]
	if !ok {
		return 0, fmt.Errorf("trigger object type %s not found in data model: %w", scenario.TriggerObjectType, models.NotFoundError)
	}

	// list objects to score
	numberOfCreatedDecisions := 0
	err = usecase.TransactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		var objects []models.ClientObject
		err = usecase.OrgTransactionFactory.TransactionInOrgSchema(scenario.OrganizationId, func(clientTx repositories.Transaction) error {
			objects, err = usecase.IngestedDataReadRepository.ListAllObjectsFromTable(clientTx, table)
			return err
		})
		if err != nil {
			return err
		}

		// execute scenario for each object
		for _, object := range objects {
			scenarioExecution, err := evaluate_scenario.EvalScenario(
				ctx,
				evaluate_scenario.ScenarioEvaluationParameters{
					Scenario:  scenario,
					Payload:   object,
					DataModel: dataModel,
				},
				evaluate_scenario.ScenarioEvaluationRepositories{
					ScenarioIterationReadRepository: usecase.ScenarioIterationReadRepository,
					OrgTransactionFactory:           usecase.OrgTransactionFactory,
					IngestedDataReadRepository:      usecase.IngestedDataReadRepository,
					EvaluateRuleAstExpression:       usecase.EvaluateRuleAstExpression,
				},
				utils.LoggerFromContext(ctx),
			)

			if errors.Is(err, models.ScenarioTriggerConditionAndTriggerObjectMismatchError) {
				continue
			} else if err != nil {
				return fmt.Errorf("error evaluating scenario: %w", err)
			}

			decisionInput := models.Decision{
				ClientObject:         object,
				Outcome:              scenarioExecution.Outcome,
				ScenarioId:           scenarioExecution.ScenarioId,
				ScenarioName:         scenarioExecution.ScenarioName,
				ScenarioDescription:  scenarioExecution.ScenarioDescription,
				ScenarioVersion:      scenarioExecution.ScenarioVersion,
				RuleExecutions:       scenarioExecution.RuleExecutions,
				Score:                scenarioExecution.Score,
				ScheduledExecutionId: &scheduledExecutionId,
			}

			err = usecase.DecisionRepository.StoreDecision(tx, decisionInput, scenario.OrganizationId, utils.NewPrimaryKey(scenario.OrganizationId))
			if err != nil {
				return fmt.Errorf("error storing decision: %w", err)
			}
			numberOfCreatedDecisions += 1
		}
		return nil
	})
	return numberOfCreatedDecisions, err
}

func (usecase *RunScheduledExecution) getPublishedScenarioIteration(scenario models.Scenario) (*models.PublishedScenarioIteration, error) {
	if scenario.LiveVersionID == nil {
		return nil, nil
	}

	liveVersion, err := usecase.ScenarioIterationReadRepository.GetScenarioIteration(nil, *scenario.LiveVersionID)
	if err != nil {
		return nil, err
	}
	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return nil, err
	}
	return &publishedVersion, nil
}
