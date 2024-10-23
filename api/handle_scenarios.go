package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func listScenarios(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()
		scenarios, err := usecase.ListScenarios(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(scenarios, dto.AdaptScenarioDto))
	}
}

func createScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreateScenarioBody
		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()
		scenario, err := usecase.CreateScenario(
			ctx,
			dto.AdaptCreateScenarioInput(input, organizationId))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
	}
}

func getScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("scenario_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()
		scenario, err := usecase.GetScenario(ctx, id)

		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
	}
}

func updateScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input dto.UpdateScenarioBody
		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		scenarioId := c.Param("scenario_id")

		usecase := usecasesWithCreds(ctx, uc).NewScenarioUsecase()

		scenario, err := usecase.UpdateScenario(
			ctx,
			dto.AdaptUpdateScenarioInput(scenarioId, input))
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
	}
}
