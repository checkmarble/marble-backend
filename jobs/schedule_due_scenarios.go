package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// Runs every hour at past 10min
func ScheduleDueScenarios(ctx context.Context, usecases usecases.Usecases) error {
	logger := utils.LoggerFromContext(ctx)

	logger.InfoContext(ctx, "Start scheduling scenarios")

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
		logger.DebugContext(ctx, "executing scenario",
			slog.String("scenario", scenario.Id),
			slog.String("organization", scenario.OrganizationId),
		)
		err := runScheduledExecution.ScheduleScenarioIfDue(ctx, scenario.OrganizationId, scenario.Id)
		if err != nil {
			logger.ErrorContext(ctx, "error scheduling scenario",
				slog.String("scenario", scenario.Id),
				slog.String("organization", scenario.OrganizationId),
			)
			logger.ErrorContext(ctx, fmt.Sprintf("error scheduling scenario: %+v", err))
		}
	}
	logger.InfoContext(ctx, "Done scheduling all due scenarios")
	return nil
}
