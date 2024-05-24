package jobs

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/tracing"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// Runs every hour at past 10min
func ScheduleDueScenarios(ctx context.Context, uc usecases.Usecases, config tracing.Configuration) error {
	return executeWithMonitoring(
		ctx,
		uc,
		config,
		"scenario-scheduler",
		func(
			ctx context.Context, usecases usecases.Usecases,
		) error {
			return scheduledScenarios(ctx, usecases)
		},
	)
}

func scheduledScenarios(ctx context.Context, usecases usecases.Usecases) error {
	logger := utils.LoggerFromContext(ctx)
	scenarios, err := usecases.Repositories.MarbleDbRepository.ListAllScenarios(
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
