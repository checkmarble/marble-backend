package jobs

import (
	"context"
	"log"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases) {
	logger := utils.LoggerFromContext(ctx)

	logger.InfoContext(ctx, "Executing all scheduled scenarios")
	scenarios, err := usecases.Repositories.MarbleDbRepository.ListAllScenarios(nil)

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	runScheduledExecution := usecasesWithCreds.NewRunScheduledExecution()
	if err != nil {
		log.Fatal(err)
	}
	for _, scenario := range scenarios {
		logger.DebugContext(ctx, "Executing scenario: "+scenario.Id, "scenarioId", scenario.Id)
		err := runScheduledExecution.ExecuteScheduledScenarioIfDue(ctx, scenario.OrganizationId, scenario.Id)
		if err != nil {
			logger.ErrorContext(ctx, "Error executing scheduled scenario: "+scenario.Id, "scenarioId", scenario.Id, " Error: ", err)
		}
	}
	logger.InfoContext(ctx, "Done executing all scheduled scenarios")
}
