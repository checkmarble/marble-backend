package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type APIScenario struct {
	Id                string    `json:"id"`
	OrganizationId    string    `json:"organization_id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	TriggerObjectType string    `json:"triggerObjectType"`
	CreatedAt         time.Time `json:"createdAt"`
	LiveVersionID     *string   `json:"liveVersionId,omitempty"`
}

func NewAPIScenario(scenario models.Scenario) APIScenario {
	return APIScenario{
		Id:                scenario.Id,
		OrganizationId:    scenario.OrganizationId,
		Name:              scenario.Name,
		Description:       scenario.Description,
		TriggerObjectType: scenario.TriggerObjectType,
		CreatedAt:         scenario.CreatedAt,
		LiveVersionID:     scenario.LiveVersionID,
	}
}

func (api *API) ListScenarios(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()
	scenarios, err := usecase.ListScenarios(c.Request.Context())
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, pure_utils.Map(scenarios, NewAPIScenario))
}

func (api *API) CreateScenario(c *gin.Context) {
	var input dto.CreateScenarioBody
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()
	scenario, err := usecase.CreateScenario(c.Request.Context(), dto.AdaptCreateScenario(input))
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, NewAPIScenario(scenario))
}

func (api *API) GetScenario(c *gin.Context) {
	id := c.Param("scenario_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()
	scenario, err := usecase.GetScenario(c.Request.Context(), id)

	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, NewAPIScenario(scenario))
}

func (api *API) UpdateScenario(c *gin.Context) {
	var input dto.UpdateScenarioBody
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	scenarioID := c.Param("scenario_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()
	scenario, err := usecase.UpdateScenario(c.Request.Context(), models.UpdateScenarioInput{
		Id:          scenarioID,
		Name:        input.Name,
		Description: input.Description,
	})
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, NewAPIScenario(scenario))
}
