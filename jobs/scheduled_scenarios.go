package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

const batchScenarioExecutionTimeout = 3 * time.Hour

func ExecuteAllScheduledScenarios(ctx context.Context, uc usecases.Usecaser) {
	executeWithMonitoring(
		ctx,
		uc,
		"scheduled-execution",
		func(
			ctx context.Context, usecases usecases.Usecaser,
		) error {
			usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
			runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()
			ctx, cancel := context.WithTimeout(ctx, batchScenarioExecutionTimeout)
			defer cancel()

			// First, schedule all due scenarios
			err := scheduledScenarios(ctx, usecases)
			if err != nil {
				return err
			}

			// Then, execute all scheduled scenarios
			err = runScheduledExecution.ExecuteAllScheduledScenarios(ctx)
			if err != nil {
				return err
			}

			return nil
		},
	)
}

func scheduledScenarios(ctx context.Context, usecases usecases.Usecaser) error {
	logger := utils.LoggerFromContext(ctx)
	scenarios, err := usecases.GetRepositories().MarbleDbRepository.ListAllScenarios(
		ctx,
		usecases.NewExecutorFactory().NewExecutor(),
		models.ListAllScenariosFilters{Live: true},
	)
	if err != nil {
		return errors.Wrap(err, "Error while listing all live scenarios")
	}

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()

	for _, scenario := range scenarios {
		logger.DebugContext(ctx, "Scheduled scenario",
			slog.String("scenario", scenario.Id),
			slog.String("organization", scenario.OrganizationId),
		)
		err := runScheduledExecution.ScheduleScenarioIfDue(ctx, scenario.OrganizationId, scenario.Id)
		if err != nil {
			return err
		}
	}
	logger.InfoContext(ctx, "Done scheduling all due scenarios")
	return nil
}
