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
	scenarioUsecase := usecases.NewScenarioUsecase()
	scenarios, err := scenarioUsecase.ListAllScenarios()

	usecase := usecases.NewScheduledExecutionUsecase()
	if err != nil {
		log.Fatal(err)
	}
	logger := utils.LoggerFromContext(ctx)
	for _, scenario := range scenarios {
		logger.DebugCtx(ctx, "Executing scenario: "+scenario.ID, "scenarioID", scenario.ID)
		err := usecase.ExecuteScheduledScenarioIfDue(ctx, scenario.OrganizationID, scenario.ID)
		if err != nil {
			logger.ErrorCtx(ctx, "Error executing scheduled scenario: "+scenario.ID, "scenarioId", scenario.ID, " Error: ", err)
		}
	}
	logger.InfoCtx(ctx, "Done executing all scheduled scenarios")
}
