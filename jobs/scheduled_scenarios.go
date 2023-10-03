package jobs

import (
	"context"
	"log"
	"log/slog"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases) {
	logger := utils.LoggerFromContext(ctx)

	logger.InfoContext(ctx, "Executing all scheduled scenarios")
	scenarios, err := usecases.Repositories.MarbleDbRepository.ListAllScenarios(nil)
	if err != nil {
		log.Fatal(err)
	}

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()
	if err != nil {
		log.Fatal(err)
	}
	for _, scenario := range scenarios {
		logger.DebugContext(ctx, "executing scenario",
			slog.String("scenario", scenario.Id),
			slog.String("organization", scenario.OrganizationId),
		)
		err := runScheduledExecution.ExecuteScheduledScenarioIfDue(ctx, scenario.OrganizationId, scenario.Id)
		if err != nil {
			logger.ErrorContext(ctx, "error executing scheduled scenario",
				slog.String("scenario", scenario.Id),
				slog.String("organization", scenario.OrganizationId),
				slog.String("error", err.Error()),
			)
		}
	}
	logger.InfoContext(ctx, "Done executing all scheduled scenarios")
}
