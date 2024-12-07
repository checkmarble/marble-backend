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
		testrunId := c.Param("test_run_id")
		if testrunId == "" {
			c.Status(http.StatusBadRequest)
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewDecisionUsecase()
		decisions, err := usecase.GetDecisionsByOutcomeAndScore(ctx, testrunId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"decisions": dto.ProcessDecisionDataDtoFromModels(decisions)})
	}
}

func handleListRulesExecution(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		testrunId := c.Param("test_run_id")
		if testrunId == "" {
			c.Status(http.StatusBadRequest)
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewRuleUsecase()
		rules, err := usecase.ListRuleExecution(ctx, testrunId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{"rules": dto.ProcessRuleExecutionDataDtoFromModels(rules)})
	}
}
