package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/usecases"
)

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
			return runScheduledExecution.ExecuteAllScheduledScenarios(ctx)
		},
	)
}
