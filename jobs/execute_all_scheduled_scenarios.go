package jobs

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/usecases"
)

const batchScenarioExecutionTimeout = 3 * time.Hour

// Runs every minute
func ExecuteAllScheduledScenarios(ctx context.Context, uc usecases.Usecases) error {
	return executeWithMonitoring(
		ctx,
		uc,
		"scheduled-execution",
		func(
			ctx context.Context, usecases usecases.Usecases,
		) error {
			usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
			runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()
			ctx, cancel := context.WithTimeout(ctx, batchScenarioExecutionTimeout)
			defer cancel()
			return runScheduledExecution.ExecuteAllScheduledScenarios(ctx)
		},
	)
}
