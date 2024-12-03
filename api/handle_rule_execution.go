package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleListRulesExecution(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		testrunId := c.Param("testrun_id")
		if testrunId == "" {
			c.Status(http.StatusBadRequest)
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewRuleUsecase()
		rules, err := usecase.ListRuleExecution(ctx, testrunId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.ProcessRuleExecutionDataDtoFromModels(rules))
	}
}
