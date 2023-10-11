package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// Runs every hour at past 10min
func ScheduleDueScenarios(ctx context.Context, usecases usecases.Usecases) error {
	logger := utils.LoggerFromContext(ctx)

	logger.InfoContext(ctx, "Start scheduling scenarios")

	scenarios, err := usecases.Repositories.MarbleDbRepository.ListAllScenarios(nil, models.ListAllScenariosFilters{Live: true})
	if err != nil {
		return fmt.Errorf("Error while listing all live scenarios: %w", err)
	}

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()
	if err != nil {
		return fmt.Errorf("usecasesWithCreds.NewRunScheduledExecution error: %w", err)
	}

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
				slog.String("error", err.Error()),
			)
		}
	}
	logger.InfoContext(ctx, "Done scheduling all due scenarios")
	return nil
}
