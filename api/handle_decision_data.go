package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleDecisionsDataByOutcomeAndScore(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		testrunId := c.Param("testrun_id")
		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, err := usecase.GetDecisionsByOutcomeAndScore(ctx, testrunId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.ProcessDecisionDataDtoFromModels(decisions))
	}
}
