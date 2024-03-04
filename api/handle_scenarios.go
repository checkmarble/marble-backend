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
	Id                     string    `json:"id"`
	CreatedAt              time.Time `json:"createdAt"`
	DecisionToCaseOutcomes []string  `json:"decision_to_case_outcomes"`
	DecisionToCaseInboxId  string    `json:"decision_to_case_inbox_id"`
	Description            string    `json:"description"`
	LiveVersionID          *string   `json:"liveVersionId,omitempty"`
	Name                   string    `json:"name"`
	OrganizationId         string    `json:"organization_id"`
	TriggerObjectType      string    `json:"triggerObjectType"`
}

func NewAPIScenario(scenario models.Scenario) APIScenario {
	out := APIScenario{
		Id:        scenario.Id,
		CreatedAt: scenario.CreatedAt,
		DecisionToCaseOutcomes: pure_utils.Map(scenario.DecisionToCaseOutcomes,
			func(o models.Outcome) string { return o.String() }),
		Description:       scenario.Description,
		LiveVersionID:     scenario.LiveVersionID,
		Name:              scenario.Name,
		OrganizationId:    scenario.OrganizationId,
		TriggerObjectType: scenario.TriggerObjectType,
	}
	if scenario.DecisionToCaseInboxId != nil {
		out.DecisionToCaseInboxId = *scenario.DecisionToCaseInboxId
	}
	return out
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
	scenarioId := c.Param("scenario_id")

	usecase := api.UsecasesWithCreds(c.Request).NewScenarioUsecase()

	scenario, err := usecase.UpdateScenario(
		c.Request.Context(),
		dto.AdaptUpdateScenario(scenarioId, input))
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, NewAPIScenario(scenario))
}
