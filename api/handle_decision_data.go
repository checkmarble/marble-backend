package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleDecisionsData(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioID := c.Param("scenario_id")

		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, err := usecase.GetDecisionsByVersionByOutcome(ctx, scenarioID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.ProcessDecisionDataDtoFromModels(decisions))
	}
}
