package usecases

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"github.com/adhocore/gronx"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/org_transaction"
	"github.com/checkmarble/marble-backend/usecases/scheduledexecution"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

type ScheduledExecutionUsecase struct {
	enforceSecurity                 security.EnforceSecurityDecision
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	scenarioPublicationsRepository  repositories.ScenarioPublicationRepository
	scheduledExecutionRepository    repositories.ScheduledExecutionRepository
	dataModelRepository             repositories.DataModelRepository
	transactionFactory              repositories.TransactionFactory
	orgTransactionFactory           org_transaction.Factory
	ingestedDataReadRepository      repositories.IngestedDataReadRepository
	decisionRepository              repositories.DecisionRepository
	customListRepository            repositories.CustomListRepository
	exportScheduleExecution         scheduledexecution.ExportScheduleExecution
	evaluateRuleAstExpression       ast_eval.EvaluateRuleAstExpression
}

func (usecase *ScheduledExecutionUsecase) GetScheduledExecution(id string) (models.ScheduledExecution, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.ScheduledExecution, error) {
		execution, err := usecase.scheduledExecutionRepository.GetScheduledExecution(tx, id)
		if err != nil {
			return models.ScheduledExecution{}, err
		}
		if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
			return models.ScheduledExecution{}, err
		}
		return execution, nil
	})
}

func (usecase *ScheduledExecutionUsecase) ExportScheduledExecutionDecisions(scheduledExecutionID string, w io.Writer) (int, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (int, error) {
		execution, err := usecase.scheduledExecutionRepository.GetScheduledExecution(tx, scheduledExecutionID)
		if err != nil {
			return 0, err
		}
		if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
			return 0, err
		}

		return usecase.exportScheduleExecution.ExportDecisions(execution.Id, w)
	})
}

func (usecase *ScheduledExecutionUsecase) ListScheduledExecutions(organizationId string, scenarioId string) ([]models.ScheduledExecution, error) {
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) ([]models.ScheduledExecution, error) {
		executions, err := usecase.scheduledExecutionRepository.ListScheduledExecutions(tx, organizationId, scenarioId)
		if err != nil {
			return []models.ScheduledExecution{}, err
		}
		for _, execution := range executions {
			if err := usecase.enforceSecurity.ReadScheduledExecution(execution); err != nil {
				return []models.ScheduledExecution{}, err
			}
		}
		return executions, nil
	})
}

func (usecase *ScheduledExecutionUsecase) CreateScheduledExecution(input models.CreateScheduledExecutionInput) error {
	if err := usecase.enforceSecurity.CreateScheduledExecution(input.OrganizationId); err != nil {
		return err
	}
	id := utils.NewPrimaryKey(input.OrganizationId)
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.scheduledExecutionRepository.CreateScheduledExecution(tx, input, id)
	})
}

func (usecase *ScheduledExecutionUsecase) UpdateScheduledExecution(ctx context.Context, input models.UpdateScheduledExecutionInput) error {

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		execution, err := usecase.scheduledExecutionRepository.GetScheduledExecution(tx, input.Id)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.CreateScheduledExecution(execution.OrganizationId); err != nil {
			return err
		}
		return usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, input)
	})
}

func (usecase *ScheduledExecutionUsecase) ExecuteScheduledScenarioIfDue(ctx context.Context, organizationId string, scenarioId string) (err error) {
	logger := utils.LoggerFromContext(ctx)
	// This is called by a cron job, for all scheduled scenarios. It is crucial that a panic on one scenario does not break all the others.
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorContext(ctx, "recovered from panic during scheduled scenario execution. Stacktrace from panic: ")
			logger.ErrorContext(ctx, string(debug.Stack()))
			err = fmt.Errorf("recovered from panic during scheduled scenario execution")
		}
	}()

	scenario, err := usecase.scenarioReadRepository.GetScenarioById(nil, scenarioId)
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

		scheduledExecution, err := repositories.TransactionReturnValue(
			usecase.transactionFactory,
			models.DATABASE_MARBLE_SCHEMA,
			func(tx repositories.Transaction) (models.ScheduledExecution, error) {
				scheduledExecutionId := utils.NewPrimaryKey(organizationId)
				if err := usecase.scheduledExecutionRepository.CreateScheduledExecution(tx, models.CreateScheduledExecutionInput{
					OrganizationId:      organizationId,
					ScenarioId:          scenarioId,
					ScenarioIterationId: publishedVersion.Id,
				}, scheduledExecutionId); err != nil {
					return models.ScheduledExecution{}, err
				}

				if err := usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, models.UpdateScheduledExecutionInput{
					Id:     scheduledExecutionId,
					Status: utils.PtrTo(models.ScheduledExecutionFailure, nil),
				}); err != nil {
					return models.ScheduledExecution{}, err
				}

				// Actually execute the scheduled scenario
				if err := usecase.executeScheduledScenario(ctx, scheduledExecutionId, scenario); err != nil {
					return models.ScheduledExecution{}, err
				}

				if err := usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, models.UpdateScheduledExecutionInput{
					Id:     scheduledExecutionId,
					Status: utils.PtrTo(models.ScheduledExecutionSuccess, nil),
				}); err != nil {
					return models.ScheduledExecution{}, err
				}
				// Mark the scheduled scenario as sucess
				logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s executed successfully", publishedVersion.Id))

				return usecase.scheduledExecutionRepository.GetScheduledExecution(tx, scheduledExecutionId)
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

func (usecase *ScheduledExecutionUsecase) scenarioIsDue(ctx context.Context, publishedVersion models.PublishedScenarioIteration, scenario models.Scenario) (bool, error) {
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
	previousExecutions, err := usecase.ListScheduledExecutions(scenario.OrganizationId, scenario.Id)
	if err != nil {
		return false, fmt.Errorf("error listing scheduled executions: %w", err)
	}

	publications, err := usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(nil, scenario.OrganizationId, models.ListScenarioPublicationsFilters{ScenarioId: &scenario.Id})
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

func (usecase *ScheduledExecutionUsecase) executeScheduledScenario(ctx context.Context, scheduledExecutionId string, scenario models.Scenario) error {
	dataModel, err := usecase.dataModelRepository.GetDataModel(nil, scenario.OrganizationId)
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
		err = usecase.orgTransactionFactory.TransactionInOrgSchema(scenario.OrganizationId, func(clientTx repositories.Transaction) error {
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
					customListRepository:            usecase.customListRepository,
					evaluateRuleAstExpression:       usecase.evaluateRuleAstExpression,
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

			err = usecase.decisionRepository.StoreDecision(tx, decisionInput, scenario.OrganizationId, utils.NewPrimaryKey(scenario.OrganizationId))
			if err != nil {
				return fmt.Errorf("error storing decision: %w", err)
			}
		}
		return nil
	})
	return err
}

func (usecase *ScheduledExecutionUsecase) getPublishedScenarioIteration(scenario models.Scenario) (*models.PublishedScenarioIteration, error) {
	if scenario.LiveVersionID == nil {
		return nil, nil
	}

	liveVersion, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(nil, *scenario.LiveVersionID)
	if err != nil {
		return nil, err
	}
	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return nil, err
	}
	return &publishedVersion, nil
}
