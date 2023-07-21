package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
)

type APIScenario struct {
	ID                string    `json:"id"`
	OrganizationId    string    `json:"organization_id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	TriggerObjectType string    `json:"triggerObjectType"`
	CreatedAt         time.Time `json:"createdAt"`
	LiveVersionID     *string   `json:"liveVersionId,omitempty"`
}

func NewAPIScenario(scenario models.Scenario) APIScenario {
	return APIScenario{
		ID:                scenario.ID,
		OrganizationId:    scenario.OrganizationID,
		Name:              scenario.Name,
		Description:       scenario.Description,
		TriggerObjectType: scenario.TriggerObjectType,
		CreatedAt:         scenario.CreatedAt,
		LiveVersionID:     scenario.LiveVersionID,
	}
}

func (api *API) ListScenarios() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		usecase := api.UsecasesWithCreds(r).NewScenarioUsecase()
		scenarios, err := usecase.ListScenarios()
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, utils.Map(scenarios, NewAPIScenario))
	}
}

func (api *API) CreateScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateScenarioInput)

		usecase := api.UsecasesWithCreds(r).NewScenarioUsecase()
		scenario, err := usecase.CreateScenario(dto.AdaptCreateScenario(input, orgID))
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, NewAPIScenario(scenario))
	}
}

type GetScenarioInput struct {
	ScenarioID string `in:"path=scenarioID"`
}

func (api *API) GetScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		input := r.Context().Value(httpin.Input).(*GetScenarioInput)

		usecase := api.UsecasesWithCreds(r).NewScenarioUsecase()
		scenario, err := usecase.GetScenario(input.ScenarioID)

		if presentError(w, r, err) {
			return
		}
		PresentModel(w, NewAPIScenario(scenario))
	}
}

func (api *API) UpdateScenario() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := r.Context().Value(httpin.Input).(*dto.UpdateScenarioInput)
		usecase := api.UsecasesWithCreds(r).NewScenarioUsecase()
		scenario, err := usecase.UpdateScenario(models.UpdateScenarioInput{
			ID:          input.ScenarioID,
			Name:        input.Body.Name,
			Description: input.Body.Description,
		})
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, NewAPIScenario(scenario))
	}
}
