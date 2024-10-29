package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func handleScenarioTestRun(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var data dto.CreateScenarioTestRunBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewScenarioTestRunUseCase()
		input, err := dto.AdaptCreateScenarioTestRunBody(data)
		if presentError(ctx, c, err) {
			return
		}
		scenarioTestRun, err := usecase.ActivateScenarioTestRun(ctx, organizationId, input)
		if presentError(ctx, c, err) {
			return
		}
		result := dto.AdaptScenarioTestRunDto(scenarioTestRun)
		c.JSON(http.StatusOK, result)
	}
}
