package jobs

import (
	"context"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/usecases"
)

// Runs every minute
func ExecuteAllScheduledScenarios(ctx context.Context, uc usecases.Usecases, config infra.TelemetryConfiguration) error {
	return executeWithMonitoring(
		ctx,
		uc,
		config,
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
