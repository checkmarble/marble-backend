package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func handleCreateScenarioTestRun(uc usecases.Usecases) func(c *gin.Context) {
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
		c.JSON(http.StatusOK, gin.H{
			"scenario_test_run": result,
		})
	}
}

func handleListScenarioTestRun(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioID := c.Query("scenario_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioTestRunUseCase()
		testruns, err := usecase.ListTestRunByScenarioId(ctx, scenarioID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(testruns, dto.AdaptScenarioTestRunDto))
	}
}

func handleGetScenarioTestRun(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		testRunId := c.Param("test_run_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioTestRunUseCase()
		testrun, err := usecase.GetTestRunById(ctx, testRunId)
		if presentError(ctx, c, err) {
			return
		}
		result := dto.AdaptScenarioTestRunDto(testrun)
		c.JSON(http.StatusOK, gin.H{
			"scenario_test_run": result,
		})
	}
}
