package jobs

import (
	"context"
	"log"
	"marble/marble-backend/usecases"

	"golang.org/x/exp/slog"
)

func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases, logger *slog.Logger) {

	scenarioUsecase := usecases.NewScenarioUsecase()
	scenarios, err := scenarioUsecase.ListAllScenarios(ctx)

	usecase := usecases.NewScheduledExecutionUsecase()
	if err != nil {
		log.Fatal(err)
	}
	for _, scenario := range scenarios {
		usecase.ExecuteScheduledScenarioIfDue(ctx, scenario.OrganizationID, scenario.ID, logger)
	}
}
