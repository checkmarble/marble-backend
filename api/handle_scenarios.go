package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
)

func (api *API) ListScenarios(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()
	scenarios, err := usecase.ListScenarios(c.Request.Context())
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, pure_utils.Map(scenarios, dto.AdaptScenarioDto))
}

func (api *API) CreateScenario(c *gin.Context) {
	var input dto.CreateScenarioBody
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()
	scenario, err := usecase.CreateScenario(c.Request.Context(), dto.AdaptCreateScenarioInput(input))
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
}

func (api *API) GetScenario(c *gin.Context) {
	id := c.Param("scenario_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()
	scenario, err := usecase.GetScenario(c.Request.Context(), id)

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
}

func (api *API) UpdateScenario(c *gin.Context) {
	var input dto.UpdateScenarioBody
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	scenarioId := c.Param("scenario_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()

	scenario, err := usecase.UpdateScenario(
		c.Request.Context(),
		dto.AdaptUpdateScenarioInput(scenarioId, input))
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, dto.AdaptScenarioDto(scenario))
}
