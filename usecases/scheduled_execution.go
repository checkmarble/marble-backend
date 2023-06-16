package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/utils"
	"runtime/debug"
	"time"

	"github.com/adhocore/gronx"
	"github.com/google/uuid"
	"golang.org/x/exp/slog"
)

type ScheduledExecutionUsecase struct {
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	scheduledExecutionRepository    repositories.ScheduledExecutionRepository
	transactionFactory              repositories.TransactionFactory
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
	id := uuid.NewString()
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.scheduledExecutionRepository.CreateScheduledExecution(tx, input, id)
	})
}

func (usecase *ScheduledExecutionUsecase) UpdateScheduledExecution(ctx context.Context, input models.UpdateScheduledExecutionInput) error {
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, input)
	})
}

func (usecase *ScheduledExecutionUsecase) ExecuteScheduledScenarioIfDue(ctx context.Context, orgID string, scenarioID string, logger *slog.Logger) (err error) {
	// This is called by a cron job, for all scheduled scenarios. It is crucial that a panic on one scenario does not break all the others.
	defer func() {
		if r := recover(); r != nil {
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

	tz, _ := time.LoadLocation("Europe/Paris")
	isDue, err := executionIsDue(publishedVersion.Body.Schedule, previousExecutions, tz)
	if err != nil {
		return err
	}

	if isDue {
		logger.DebugCtx(ctx, fmt.Sprintf("Scenario iteration %s is due", publishedVersion.ID))
		id := uuid.NewString()
		err = usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
			return usecase.scheduledExecutionRepository.CreateScheduledExecution(tx, models.CreateScheduledExecutionInput{
				OrganizationID:      orgID,
				ScenarioID:          scenarioID,
				ScenarioIterationID: publishedVersion.ID,
			}, id)
		})
		if err != nil {
			return err
		}

		// Actually execute the scheduled scenario
		if err := executeScheduledBatchScenario(ctx, usecase, orgID, scenarioID, publishedVersion, logger); err != nil {
			usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
				return usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, models.UpdateScheduledExecutionInput{
					ID:     id,
					Status: utils.PtrTo("failure", nil),
				})
			})
			return err
		}

		// Mark the scheduled scenario as sucess
		err = usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
			return usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, models.UpdateScheduledExecutionInput{
				ID:     id,
				Status: utils.PtrTo("success", nil),
			})
		})
		if err != nil {
			return err
		}
		logger.DebugCtx(ctx, fmt.Sprintf("Scenario iteration %s executed successfully", publishedVersion.ID))
		return nil
	} else {

		return nil
	}
}

func executionIsDue(schedule string, previousExecutions []models.ScheduledExecution, tz *time.Location) (bool, error) {
	if len(previousExecutions) == 0 {
		return true, nil
	}

	nextTick, err := gronx.NextTickAfter(schedule, previousExecutions[0].StartedAt.In(tz), false)
	if err != nil {
		return true, err
	}
	if nextTick.After(time.Now()) {
		return false, nil
	}
	return true, nil
}

func executeScheduledBatchScenario(ctx context.Context, usecase *ScheduledExecutionUsecase, orgID, scenarioID string, publishedVersion models.PublishedScenarioIteration, logger *slog.Logger) error {
	return fmt.Errorf("Not implemented")
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
