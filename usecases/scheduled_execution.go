package usecases

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"runtime/debug"

	"github.com/adhocore/gronx"
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
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.scheduledExecutionRepository.CreateScheduledExecution(tx, input)
	})
}

func (usecase *ScheduledExecutionUsecase) UpdateScheduledExecution(ctx context.Context, input models.UpdateScheduledExecutionInput) error {
	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		return usecase.scheduledExecutionRepository.UpdateScheduledExecution(tx, input)
	})
}

func (usecase *ScheduledExecutionUsecase) ExecuteScheduledScenarioIfDue(ctx context.Context, orgID string, scenarioID string, logger *slog.Logger) error {
	// This is called by a cron job, for all scheduled scenarios. It is crucial that a panic on one scenario does not break all the others.
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorCtx(ctx, "recovered from panic during scheduled scenario execution. Stacktrace from panic: ")
			logger.ErrorCtx(ctx, string(debug.Stack()))
		}
	}()

	scenario, err := usecase.scenarioReadRepository.GetScenario(ctx, orgID, scenarioID)
	if err != nil {
		return err
	}
	if scenario.LiveVersionID == nil {
		return fmt.Errorf("Scenario has no live version %w", models.BadParameterError)
	}
	scenarioIteration, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(ctx, orgID, *scenario.LiveVersionID)
	if err != nil {
		return err
	}
	if scenarioIteration.Body.Schedule == "" {
		return fmt.Errorf("Scenario is not scheduled %w", models.BadParameterError)
	}

	liveVersion, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(ctx, orgID, *scenario.LiveVersionID)
	if err != nil {
		return err
	}
	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
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

	isDue, err := gron.IsDue(publishedVersion.Body.Schedule)
	if err != nil {
		return err
	}
	if len(previousExecutions) > 0 {
		prevIsDue, err := gron.IsDue(publishedVersion.Body.Schedule, previousExecutions[0].StartedAt)
		if err != nil {
			return err
		}
		isDue = isDue && !prevIsDue
	}

	if isDue {
		// TODO: implement batch scenar execution/dispatch
	} else {
		// Scheduled scenario has already been executed (or is in the process of being executed)
		return nil
	}

	return nil
}
