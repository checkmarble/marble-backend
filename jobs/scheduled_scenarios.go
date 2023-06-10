package jobs

import (
	"context"
	"marble/marble-backend/usecases"

	"golang.org/x/exp/slog"
)

type ScheduledScenario struct {
	OrganizationID string
	ScenarioID     string
}

func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases, logger *slog.Logger) {
	usecase := usecases.NewScheduledScenarioExecutionUsecase()

	// TODO: implement dynamic reading of it in a dedicated usecase method & repository
	listToTrigger := []ScheduledScenario{}
	for _, scenario := range listToTrigger {
		usecase.ExecuteScheduledScenarioIfDue(ctx, scenario.OrganizationID, scenario.ScenarioID, logger)
	}
}
