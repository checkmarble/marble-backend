package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleDecisionsDataByOutcome(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input dto.DecisionQuery

		errBind := c.ShouldBindQuery(&input)
		if errBind != nil {
			presentError(ctx, c, errBind)
		}
		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, err := usecase.GetDecisionsByVersionByOutcome(ctx, input.ScenarioID, input.TestRunBegin, input.TestRunEnd)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.ProcessDecisionDataDtoFromModels(decisions))
	}
}

func handleDecisionsDataByScore(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input dto.DecisionQuery

		errBind := c.ShouldBindQuery(&input)
		if errBind != nil {
			presentError(ctx, c, errBind)
		}
		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, err := usecase.GetDecisionsByVersionByScore(ctx, input.ScenarioID, input.TestRunBegin, input.TestRunEnd)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.ProcessDecisionDataDtoFromModels(decisions))
	}
}
