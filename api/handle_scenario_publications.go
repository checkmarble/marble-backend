package api

import (
	"net/http"
	"time"

	"github.com/ggicci/httpin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type APIScenarioPublication struct {
	Id                  string    `json:"id"`
	Rank                int32     `json:"rank"`
	ScenarioId          string    `json:"scenarioID"`
	ScenarioIterationId string    `json:"scenarioIterationID"`
	PublicationAction   string    `json:"publicationAction"`
	CreatedAt           time.Time `json:"createdAt"`
}

func NewAPIScenarioPublication(sp models.ScenarioPublication) APIScenarioPublication {
	return APIScenarioPublication{
		Id:                  sp.Id,
		Rank:                sp.Rank,
		ScenarioId:          sp.ScenarioId,
		ScenarioIterationId: sp.ScenarioIterationId,
		PublicationAction:   sp.PublicationAction.String(),
		CreatedAt:           sp.CreatedAt,
	}
}

func (api *API) ListScenarioPublications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := r.Context().Value(httpin.Input).(*dto.ListScenarioPublicationsInput)

		usecase := api.UsecasesWithCreds(r).NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.ListScenarioPublications(models.ListScenarioPublicationsFilters{
			ScenarioId:          input.ScenarioId,
			ScenarioIterationId: input.ScenarioIterationId,
		})
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, utils.Map(scenarioPublications, NewAPIScenarioPublication))
	}
}

func (api *API) CreateScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		input := ctx.Value(httpin.Input).(*dto.CreateScenarioPublicationInput)

		usecase := api.UsecasesWithCreds(r).NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.ExecuteScenarioPublicationAction(models.PublishScenarioIterationInput{
			ScenarioIterationId: input.Body.ScenarioIterationId,
			PublicationAction:   models.PublicationActionFrom(input.Body.PublicationAction),
		})
		if presentError(w, r, err) {
			return
		}

		PresentModel(w, utils.Map(scenarioPublications, NewAPIScenarioPublication))
	}
}

type GetScenarioPublicationInput struct {
	ScenarioPublicationID string `in:"path=scenarioPublicationID"`
}

func (api *API) GetScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := r.Context().Value(httpin.Input).(*GetScenarioPublicationInput)

		usecase := api.UsecasesWithCreds(r).NewScenarioPublicationUsecase()
		scenarioPublication, err := usecase.GetScenarioPublication(input.ScenarioPublicationID)
		if presentError(w, r, err) {
			return
		}
		PresentModel(w, NewAPIScenarioPublication(scenarioPublication))
	}
}
