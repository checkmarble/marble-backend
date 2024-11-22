package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleDecisionsData(uc usecases.Usecases, marbleAppHost string) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		decisionID := c.Param("decision_id")

		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decision, err := usecase.GetDecision(ctx, decisionID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.NewDecisionWithRuleDto(decision, marbleAppHost, true))
	}
}
