package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
)

func listScenarios(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		usecase := usecasesWithCreds(c.Request, uc).NewScenarioUsecase()
		scenarios, err := usecase.ListScenarios(c.Request.Context())
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, pure_utils.Map(scenarios, dto.AdaptScenarioDto))
	}
}

func createScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		var input dto.CreateScenarioBody
		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewScenarioUsecase()
		scenario, err := usecase.CreateScenario(c.Request.Context(), dto.AdaptCreateScenarioInput(input))
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
	}
}

func getScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("scenario_id")

		usecase := usecasesWithCreds(c.Request, uc).NewScenarioUsecase()
		scenario, err := usecase.GetScenario(c.Request.Context(), id)

		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
	}
}

func updateScenario(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		var input dto.UpdateScenarioBody
		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		scenarioId := c.Param("scenario_id")

		usecase := usecasesWithCreds(c.Request, uc).NewScenarioUsecase()

		scenario, err := usecase.UpdateScenario(
			c.Request.Context(),
			dto.AdaptUpdateScenarioInput(scenarioId, input))
		if presentError(c, err) {
			return
		}
		c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
	}
}
