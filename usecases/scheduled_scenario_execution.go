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

type ScheduledScenarioExecutionUsecase struct {
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
}

func (usecase *ScheduledScenarioExecutionUsecase) GetScheduledScenarioExecution(ctx context.Context, orgID string, id string) (models.ScheduledScenarioBatchExecution, error) {
	return models.ScheduledScenarioBatchExecution{}, nil
}

func (usecase *ScheduledScenarioExecutionUsecase) ListScheduledScenarioExecutions(ctx context.Context, orgID string, scenarioID string) ([]models.ScheduledScenarioBatchExecution, error) {
	return []models.ScheduledScenarioBatchExecution{}, nil
}

func (usecase *ScheduledScenarioExecutionUsecase) UpdateScheduledScenarioExecutioExecution(ctx context.Context, orgID string, id string, input models.UpdateScheduledScenarioExecutionBody) (models.ScheduledScenarioBatchExecution, error) {
	return models.ScheduledScenarioBatchExecution{}, nil
}

func (usecase *ScheduledScenarioExecutionUsecase) ExecuteScheduledScenarioIfDue(ctx context.Context, orgID string, scenarioID string, logger *slog.Logger) error {
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
	if scenario.ScenarioType != models.Scheduled {
		return fmt.Errorf("Scenario is not scheduled %w", models.BadParameterError)
	}
	if scenario.LiveVersionID == nil {
		return fmt.Errorf("Scenario has no live version %w", models.BadParameterError)
	}

	liveVersion, err := usecase.scenarioIterationReadRepository.GetScenarioIteration(ctx, orgID, *scenario.LiveVersionID)
	if err != nil {
		return err
	}
	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion, scenario.ScenarioType)
	if err != nil {
		return err
	}

	gron := gronx.New()
	ok := gron.IsValid(publishedVersion.Body.Schedule)
	if !ok {
		return fmt.Errorf("Invalid schedule: %w", models.BadParameterError)
	}
	previousExecutions, err := usecase.ListScheduledScenarioExecutions(ctx, orgID, scenarioID)
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

func (usecase *ScheduledScenarioExecutionUsecase) GetScheduledScenarioObjectExecution(ctx context.Context, orgID string, id string) (models.ScheduledScenarioObjectExecution, error) {
	return models.ScheduledScenarioObjectExecution{}, nil
}

func (usecase *ScheduledScenarioExecutionUsecase) ListScheduledScenarioObjectExecutions(ctx context.Context, orgID string, scenarioID string) ([]models.ScheduledScenarioObjectExecution, error) {
	return []models.ScheduledScenarioObjectExecution{}, nil
}
