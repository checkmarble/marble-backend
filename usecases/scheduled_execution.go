package usecases

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"
	"marble/marble-backend/usecases/scheduledexecution"
	"marble/marble-backend/utils"
	"runtime/debug"
	"time"

	"github.com/adhocore/gronx"
)

type ScheduledExecutionUsecase struct {
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	scenarioPublicationsRepository  repositories.ScenarioPublicationRepository
	scheduledExecutionRepository    repositories.ScheduledExecutionRepository
	dataModelRepository             repositories.DataModelRepository
	transactionFactory              repositories.TransactionFactory
	orgTransactionFactory           organization.OrgTransactionFactory
	ingestedDataReadRepository      repositories.IngestedDataReadRepository
	decisionRepository              repositories.DecisionRepository
	exportScheduleExecution         scheduledexecution.ExportScheduleExecution
}

func (usecase *ScheduledExecutionUsecase) GetScheduledExecution(ctx context.Context, orgID string, id string) (models.ScheduledExecution, error) {

	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.ScheduledExecution, error) {
		execution, err := usecase.scheduledExecutionRepository.GetScheduledExecution(tx, orgID, id)
		if err != nil {
			return models.ScheduledExecution{}, err
		}
		return execution, nil
	})
}

func (usecase *ScheduledExecutionUsecase) ListScheduledExecutions(ctx context.Context, orgID string, scenarioID string) ([]models.ScheduledExecution, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) ([]models.ScheduledExecution, error) {
		executions, err := usecase.scheduledExecutionRepository.ListScheduledExecutions(tx, orgID, scenarioID)
		if err != nil {
			return []models.ScheduledExecution{}, err
		}
		return executions, nil
	})
}

func (usecase *ScheduledExecutionUsecase) CreateScheduledExecution(ctx context.Context, input models.CreateScheduledExecutionInput) error {
	id := utils.NewPrimaryKey(input.OrganizationID)
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.scheduledExecutionRepository.CreateScheduledExecution(tx, input, id)
	})
}

func (usecase *ScheduledExecutionUsecase) UpdateScheduledExecution(ctx context.Context, input models.UpdateScheduledExecutionInput) error {
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, input)
	})
}

func (usecase *ScheduledExecutionUsecase) ExecuteScheduledScenarioIfDue(ctx context.Context, orgID string, scenarioID string) (err error) {
	// This is called by a cron job, for all scheduled scenarios. It is crucial that a panic on one scenario does not break all the others.
	defer func() {
		if r := recover(); r != nil {
			logger := utils.LoggerFromContext(ctx)
			logger.ErrorCtx(ctx, "recovered from panic during scheduled scenario execution. Stacktrace from panic: ")
			logger.ErrorCtx(ctx, string(debug.Stack()))
			err = fmt.Errorf("Recovered from panic during scheduled scenario execution")
		}
	}()

	scenario, err := usecase.scenarioReadRepository.GetScenario(ctx, orgID, scenarioID)
	if err != nil {
		return err
	}

	publishedVersion, err := usecase.getPublishedScenarioIteration(ctx, scenario)
	if err != nil {
		return err
	}

	gron := gronx.New()
	ok := gron.IsValid(publishedVersion.Body.Schedule)
	if !ok {
		return fmt.Errorf("Invalid schedule: %w", models.BadParameterError)
	}
	previousExecutions, err := usecase.ListScheduledExecutions(ctx, orgID, scenarioID)
	if err != nil {
		return err
	}

	publications, err := usecase.scenarioPublicationsRepository.ListScenarioPublications(ctx, scenario.OrganizationID, models.ListScenarioPublicationsFilters{ScenarioID: &scenario.ID})
	if err != nil {
		return err
	}

	tz, _ := time.LoadLocation("Europe/Paris")
	isDue, err := executionIsDue(publishedVersion.Body.Schedule, previousExecutions, publications, tz)
	if err != nil {
		return err
	}

	if isDue || true {
		logger := utils.LoggerFromContext(ctx)
		logger.DebugCtx(ctx, fmt.Sprintf("Scenario iteration %s is due", publishedVersion.ID))

		scheduledExecution, err := repositories.TransactionReturnValue(
			usecase.transactionFactory,
			models.DATABASE_MARBLE_SCHEMA,
			func(tx repositories.Transaction) (models.ScheduledExecution, error) {
				scheduledExecutionId := utils.NewPrimaryKey(orgID)
				if err := usecase.scheduledExecutionRepository.CreateScheduledExecution(tx, models.CreateScheduledExecutionInput{
					OrganizationID:      orgID,
					ScenarioID:          scenarioID,
					ScenarioIterationID: publishedVersion.ID,
				}, scheduledExecutionId); err != nil {
					return models.ScheduledExecution{}, err
				}

				if err := usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, models.UpdateScheduledExecutionInput{
					ID:     scheduledExecutionId,
					Status: utils.PtrTo(models.ScheduledExecutionFailure, nil),
				}); err != nil {
					return models.ScheduledExecution{}, err
				}

				// Actually execute the scheduled scenario
				if err := usecase.executeScheduledScenario(ctx, scheduledExecutionId, scenario); err != nil {
					return models.ScheduledExecution{}, err
				}

				if err := usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, models.UpdateScheduledExecutionInput{
					ID:     scheduledExecutionId,
					Status: utils.PtrTo(models.ScheduledExecutionSuccess, nil),
				}); err != nil {
					return models.ScheduledExecution{}, err
				}
				// Mark the scheduled scenario as sucess
				logger.DebugCtx(ctx, fmt.Sprintf("Scenario iteration %s executed successfully", publishedVersion.ID))

				return usecase.scheduledExecutionRepository.GetScheduledExecution(tx, orgID, scheduledExecutionId)
			},
		)

		if err != nil {
			return err
		}

		// export decisions
		return usecase.exportScheduleExecution.ExportScheduledExecutionToS3(scenario, scheduledExecution)
	}

	return nil
}

func executionIsDue(schedule string, previousExecutions []models.ScheduledExecution, publications []models.ScenarioPublication, tz *time.Location) (bool, error) {
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

func (usecase *ScheduledExecutionUsecase) executeScheduledScenario(ctx context.Context, scheduledExecutionId string, scenario models.Scenario) error {
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, scenario.OrganizationID)
	if err != nil {
		return err
	}
	tables := dataModel.Tables
	table, ok := tables[models.TableName(scenario.TriggerObjectType)]
	if !ok {
		return fmt.Errorf("trigger object type %s not found in data model: %w", scenario.TriggerObjectType, models.NotFoundError)
	}

	// list objects to score
	err = usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		var objects []models.ClientObject
		err = usecase.orgTransactionFactory.TransactionInOrgSchema(scenario.OrganizationID, func(clientTx repositories.Transaction) error {
			objects, err = usecase.ingestedDataReadRepository.ListAllObjectsFromTable(clientTx, table)
			return err
		})
		if err != nil {
			return err
		}

		// execute scenario for each object
		for _, object := range objects {
			scenarioExecution, err := evalScenario(
				ctx,
				scenarioEvaluationParameters{
					scenario:  scenario,
					payload:   object,
					dataModel: dataModel,
				},
				scenarioEvaluationRepositories{
					scenarioIterationReadRepository: usecase.scenarioIterationReadRepository,
					orgTransactionFactory:           usecase.orgTransactionFactory,
					ingestedDataReadRepository:      usecase.ingestedDataReadRepository,
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
				ScenarioId:           scenarioExecution.ScenarioID,
				ScenarioName:         scenarioExecution.ScenarioName,
				ScenarioDescription:  scenarioExecution.ScenarioDescription,
				ScenarioVersion:      scenarioExecution.ScenarioVersion,
				RuleExecutions:       scenarioExecution.RuleExecutions,
				Score:                scenarioExecution.Score,
				ScheduledExecutionId: &scheduledExecutionId,
			}

			err = usecase.decisionRepository.StoreDecision(tx, decisionInput, scenario.OrganizationID, utils.NewPrimaryKey(scenario.OrganizationID))
			if err != nil {
				return fmt.Errorf("error storing decision: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// wrap up

	return nil
}

func (usecase *ScheduledExecutionUsecase) getPublishedScenarioIteration(ctx context.Context, scenario models.Scenario) (models.PublishedScenarioIteration, error) {
	if scenario.LiveVersionID == nil {
		return models.PublishedScenarioIteration{}, fmt.Errorf("Scenario has no live version %w", models.BadParameterError)
	}
	scenarioIteration, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(ctx, scenario.OrganizationID, *scenario.LiveVersionID)
	if err != nil {
		return models.PublishedScenarioIteration{}, err
	}
	if scenarioIteration.Body.Schedule == "" {
		return models.PublishedScenarioIteration{}, fmt.Errorf("Scenario is not scheduled %w", models.BadParameterError)
	}

	liveVersion, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(ctx, scenario.OrganizationID, *scenario.LiveVersionID)
	if err != nil {
		return models.PublishedScenarioIteration{}, err
	}
	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return models.PublishedScenarioIteration{}, err
	}
	return publishedVersion, nil
}
