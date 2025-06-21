package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleListWorkflowsForScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioId := c.Param("scenarioId")

		uc := usecasesWithCreds(ctx, uc)
		scenarioUsecase := uc.NewScenarioUsecase()

		rules, err := scenarioUsecase.ListWorkflowsForScenario(ctx, scenarioId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(rules, dto.AdaptWorkflow))
	}
}
