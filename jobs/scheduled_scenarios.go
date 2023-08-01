package jobs

import (
	"context"
	"fmt"
	"log"
	"marble/marble-backend/usecases"
	"marble/marble-backend/utils"
)

func ExecuteAllScheduledScenarios(ctx context.Context, usecases usecases.Usecases) {

	fmt.Println("Executing all scheduled scenarios")
	scenarios, err := usecases.Repositories.ScenarioReadRepository.ListAllScenarios(nil)

	usecase := usecases.NewScheduledExecutionUsecase()
	if err != nil {
		log.Fatal(err)
	}
	logger := utils.LoggerFromContext(ctx)
	for _, scenario := range scenarios {
		logger.DebugCtx(ctx, "Executing scenario: "+scenario.Id, "scenarioId", scenario.Id)
		err := usecase.ExecuteScheduledScenarioIfDue(ctx, scenario.OrganizationId, scenario.Id)
		if err != nil {
			logger.ErrorCtx(ctx, "Error executing scheduled scenario: "+scenario.Id, "scenarioId", scenario.Id, " Error: ", err)
		}
	}
	logger.InfoCtx(ctx, "Done executing all scheduled scenarios")
}
