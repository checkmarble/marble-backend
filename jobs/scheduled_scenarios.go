package jobs

import (
	"context"
	"fmt"
	"log"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases) {

	fmt.Println("Executing all scheduled scenarios")
	scenarios, err := usecases.Repositories.ScenarioReadRepository.ListAllScenarios(nil)

	usecasesWithCreds := GenerateUsecaseWithCredForMarbleAdmin(ctx, usecases)
	usecase := usecasesWithCreds.NewScheduledExecutionUsecase()
	if err != nil {
		log.Fatal(err)
	}
	logger := utils.LoggerFromContext(ctx)
	for _, scenario := range scenarios {
		logger.DebugContext(ctx, "Executing scenario: "+scenario.Id, "scenarioId", scenario.Id)
		err := usecase.ExecuteScheduledScenarioIfDue(ctx, scenario.OrganizationId, scenario.Id)
		if err != nil {
			logger.ErrorContext(ctx, "Error executing scheduled scenario: "+scenario.Id, "scenarioId", scenario.Id, " Error: ", err)
		}
	}
	logger.InfoContext(ctx, "Done executing all scheduled scenarios")
}
