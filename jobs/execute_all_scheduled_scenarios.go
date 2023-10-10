package jobs

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

// Runs every minute
func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases) error {
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, "Start pending scheduled executions")

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()
	err := runScheduledExecution.ExecuteAllScheduledScenarios(ctx)

	if err != nil {
		return fmt.Errorf("error executing scheduled scenarios: %w", err)
	}

	logger.InfoContext(ctx, "Done executing all due scenarios")
	return nil
}
