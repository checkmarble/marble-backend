package scheduledexecution

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

type RunScheduledExecutionRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (models.ScenarioIteration, error)

	ListScheduledExecutions(ctx context.Context, exec repositories.Executor,
		filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(ctx context.Context, exec repositories.Executor,
		input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error
	UpdateScheduledExecution(ctx context.Context, exec repositories.Executor,
		updateScheduledEx models.UpdateScheduledExecutionInput) error
	GetScheduledExecution(ctx context.Context, exec repositories.Executor, id string) (models.ScheduledExecution, error)
}

type RunScheduledExecution struct {
	Repository                     RunScheduledExecutionRepository
	ExecutorFactory                executor_factory.ExecutorFactory
	ExportScheduleExecution        ExportScheduleExecution
	ScenarioPublicationsRepository repositories.ScenarioPublicationRepository
	DataModelRepository            repositories.DataModelRepository
	IngestedDataReadRepository     repositories.IngestedDataReadRepository
	EvaluateRuleAstExpression      ast_eval.EvaluateRuleAstExpression
	DecisionRepository             repositories.DecisionRepository
	TransactionFactory             executor_factory.TransactionFactory
}

func (usecase *RunScheduledExecution) ScheduleScenarioIfDue(ctx context.Context, organizationId string, scenarioId string) error {
	exec := usecase.ExecutorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	scenario, err := usecase.Repository.GetScenarioById(ctx, exec, scenarioId)
	if err != nil {
		return err
	}

	publishedVersion, err := usecase.getPublishedScenarioIteration(ctx, exec, scenario)
	if err != nil {
		return err
	}
	if publishedVersion == nil {
		logger.DebugContext(ctx, fmt.Sprintf("scenario %s has no published version", scenarioId))
		return nil
	}

	previousExecutions, err := usecase.Repository.ListScheduledExecutions(ctx, exec, models.ListScheduledExecutionsFilters{
		ScenarioId: scenarioId, Status: []models.ScheduledExecutionStatus{
			models.ScheduledExecutionPending, models.ScheduledExecutionProcessing,
		},
	})
	if err != nil {
		return err
	}
	if len(previousExecutions) > 0 {
		logger.DebugContext(ctx, fmt.Sprintf("scenario %s has already a pending or processing scheduled execution", scenarioId))
		return nil
	}

	isDue, err := usecase.scenarioIsDue(ctx, *publishedVersion, scenario)
	if err != nil {
		return err
	}
	if !isDue {
		return nil
	}

	logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s is due", publishedVersion.Id))
	scheduledExecutionId := utils.NewPrimaryKey(organizationId)
	return usecase.Repository.CreateScheduledExecution(ctx, exec, models.CreateScheduledExecutionInput{
		OrganizationId:      organizationId,
		ScenarioId:          scenarioId,
		ScenarioIterationId: publishedVersion.Id,
		Manual:              false,
	}, scheduledExecutionId)
}

func (usecase *RunScheduledExecution) ExecuteAllScheduledScenarios(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)

	pendingScheduledExecutions, err := usecase.Repository.ListScheduledExecutions(ctx,
		usecase.ExecutorFactory.NewExecutor(), models.ListScheduledExecutionsFilters{
			Status: []models.ScheduledExecutionStatus{models.ScheduledExecutionPending},
		})
	if err != nil {
		return fmt.Errorf("Error while listing pending ScheduledExecutions: %w", err)
	}

	logger.InfoContext(ctx, fmt.Sprintf("Found %d pending scheduled executions", len(pendingScheduledExecutions)))

	var waitGroup sync.WaitGroup
	executionErrorChan := make(chan error, len(pendingScheduledExecutions))

	startScheduledExecution := func(scheduledExecution models.ScheduledExecution) {
		defer waitGroup.Done()
		if err := usecase.ExecuteScheduledScenario(ctx, logger, scheduledExecution); err != nil {
			executionErrorChan <- err
		}
	}

	for _, pendingExecution := range pendingScheduledExecutions {
		waitGroup.Add(1)
		go startScheduledExecution(pendingExecution)
	}

	waitGroup.Wait()
	close(executionErrorChan)

	executionErr := <-executionErrorChan
	return executionErr
}

func (usecase *RunScheduledExecution) ExecuteScheduledScenario(ctx context.Context,
	logger *slog.Logger, scheduledExecution models.ScheduledExecution,
) error {
	exec := usecase.ExecutorFactory.NewExecutor()
	logger.InfoContext(ctx, fmt.Sprintf("Start execution %s", scheduledExecution.Id))

	if err := usecase.Repository.UpdateScheduledExecution(ctx, exec, models.UpdateScheduledExecutionInput{
		Id:     scheduledExecution.Id,
		Status: utils.PtrTo(models.ScheduledExecutionProcessing, nil),
	}); err != nil {
		return err
	}

	scheduledExecution, err := executor_factory.TransactionReturnValue(ctx,
		usecase.TransactionFactory, func(tx repositories.Executor) (models.ScheduledExecution, error) {
			numberOfCreatedDecisions, err := usecase.executeScheduledScenario(ctx,
				scheduledExecution.Id, scheduledExecution.Scenario)
			if err != nil {
				return scheduledExecution, err
			}
			if err := usecase.Repository.UpdateScheduledExecution(ctx, tx, models.UpdateScheduledExecutionInput{
				Id:                       scheduledExecution.Id,
				Status:                   utils.PtrTo(models.ScheduledExecutionSuccess, nil),
				NumberOfCreatedDecisions: &numberOfCreatedDecisions,
			}); err != nil {
				return scheduledExecution, err
			}
			return usecase.Repository.GetScheduledExecution(ctx, tx, scheduledExecution.Id)
		})
	if err != nil {
		if err := usecase.Repository.UpdateScheduledExecution(ctx, exec, models.UpdateScheduledExecutionInput{
			Id:     scheduledExecution.Id,
			Status: utils.PtrTo(models.ScheduledExecutionFailure, nil),
		}); err != nil {
			return err
		}
		return err
	}

	logger.InfoContext(ctx, fmt.Sprintf("Execution completed for %s", scheduledExecution.Id))
	return usecase.ExportScheduleExecution.ExportScheduledExecutionToS3(ctx,
		scheduledExecution.Scenario, scheduledExecution)
}

func (usecase *RunScheduledExecution) scenarioIsDue(ctx context.Context,
	publishedVersion models.PublishedScenarioIteration, scenario models.Scenario,
) (bool, error) {
	exec := usecase.ExecutorFactory.NewExecutor()
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

	previousExecutions, err := usecase.Repository.ListScheduledExecutions(ctx, exec, models.ListScheduledExecutionsFilters{
		ScenarioId: scenario.Id, ExcludeManual: true,
	})
	if err != nil {
		return false, fmt.Errorf("error listing scheduled executions: %w", err)
	}

	publications, err := usecase.ScenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(
		ctx, exec, scenario.OrganizationId, models.ListScenarioPublicationsFilters{ScenarioId: &scenario.Id})
	if err != nil {
		return false, err
	}

	tz, _ := time.LoadLocation("Europe/Paris")
	return executionIsDueNow(publishedVersion.Body.Schedule, previousExecutions, publications, tz)
}

func executionIsDueNow(schedule string, previousExecutions []models.ScheduledExecution,
	publications []models.ScenarioPublication, tz *time.Location,
) (bool, error) {
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

func (usecase *RunScheduledExecution) executeScheduledScenario(ctx context.Context,
	scheduledExecutionId string, scenario models.Scenario,
) (int, error) {
	dataModel, err := usecase.DataModelRepository.GetDataModel(ctx,
		usecase.ExecutorFactory.NewExecutor(), scenario.OrganizationId, false)
	if err != nil {
		return 0, err
	}
	tables := dataModel.Tables
	table, ok := tables[models.TableName(scenario.TriggerObjectType)]
	if !ok {
		return 0, fmt.Errorf("trigger object type %s not found in data model: %w",
			scenario.TriggerObjectType, models.NotFoundError)
	}

	// list objects to score
	numberOfCreatedDecisions := 0
	var objects []models.ClientObject
	db, err := usecase.ExecutorFactory.NewClientDbExecutor(ctx, scenario.OrganizationId)
	if err != nil {
		return 0, err
	}
	objects, err = usecase.IngestedDataReadRepository.ListAllObjectsFromTable(ctx, db, table)
	if err != nil {
		return 0, err
	}

	err = usecase.TransactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
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
					EvalScenarioRepository:     usecase.Repository,
					ExecutorFactory:            usecase.ExecutorFactory,
					IngestedDataReadRepository: usecase.IngestedDataReadRepository,
					EvaluateRuleAstExpression:  usecase.EvaluateRuleAstExpression,
				},
				utils.LoggerFromContext(ctx),
			)

			if errors.Is(err, models.ScenarioTriggerConditionAndTriggerObjectMismatchError) {
				logger := utils.LoggerFromContext(ctx)
				logger.InfoContext(ctx, fmt.Sprintf("Trigger condition and trigger object mismatch: %s",
					err.Error()), "scenarioId", scenario.Id, "triggerObjectType",
					scenario.TriggerObjectType, "object", object)
				continue
			} else if err != nil {
				return errors.Wrap(err, fmt.Sprintf("error evaluating scenario in executeScheduledScenario %s", scenario.Id))
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

			err = usecase.DecisionRepository.StoreDecision(ctx, tx, decisionInput,
				scenario.OrganizationId, utils.NewPrimaryKey(scenario.OrganizationId))
			if err != nil {
				return fmt.Errorf("error storing decision: %w", err)
			}
			numberOfCreatedDecisions += 1
		}
		return nil
	})
	return numberOfCreatedDecisions, err
}

func (usecase *RunScheduledExecution) getPublishedScenarioIteration(
	ctx context.Context,
	exec repositories.Executor,
	scenario models.Scenario,
) (*models.PublishedScenarioIteration, error) {
	if scenario.LiveVersionID == nil {
		return nil, nil
	}

	liveVersion, err := usecase.Repository.GetScenarioIteration(ctx, exec, *scenario.LiveVersionID)
	if err != nil {
		return nil, err
	}
	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return nil, err
	}
	return &publishedVersion, nil
}
