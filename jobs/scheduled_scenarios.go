package jobs

import (
	"context"
	"fmt"
	"log"
	"marble/marble-backend/usecases"

	"golang.org/x/exp/slog"
)

func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases, logger *slog.Logger) {

	fmt.Println("Executing all scheduled scenarios")
	scenarioUsecase := usecases.NewScenarioUsecase()
	scenarios, err := scenarioUsecase.ListAllScenarios(ctx)

	usecase := usecases.NewScheduledExecutionUsecase()
	if err != nil {
		log.Fatal(err)
	}
	for _, scenario := range scenarios {
		logger.DebugCtx(ctx, "Executing scenario: "+scenario.ID, "scenarioID", scenario.ID)
		err := usecase.ExecuteScheduledScenarioIfDue(ctx, scenario.OrganizationID, scenario.ID, logger)
		if err != nil {
			logger.ErrorCtx(ctx, "Error executing scheduled scenario: "+scenario.ID, "scenarioId", scenario.ID, " Error: ", err)
		}
	}
	logger.InfoCtx(ctx, "Done executing all scheduled scenarios")
}
