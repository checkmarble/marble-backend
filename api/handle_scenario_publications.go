package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
)

type APIScenarioPublication struct {
	ID                  string    `json:"id"`
	Rank                int32     `json:"rank"`
	ScenarioID          string    `json:"scenarioID"`
	ScenarioIterationID string    `json:"scenarioIterationID"`
	PublicationAction   string    `json:"publicationAction"`
	CreatedAt           time.Time `json:"createdAt"`
}

func NewAPIScenarioPublication(sp models.ScenarioPublication) APIScenarioPublication {
	return APIScenarioPublication{
		ID:                  sp.ID,
		Rank:                sp.Rank,
		ScenarioID:          sp.ScenarioID,
		ScenarioIterationID: sp.ScenarioIterationID,
		PublicationAction:   sp.PublicationAction.String(),
		CreatedAt:           sp.CreatedAt,
	}
}

func (api *API) ListScenarioPublications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := r.Context().Value(httpin.Input).(*dto.ListScenarioPublicationsInput)

		options := &utils.PtrToOptions{OmitZero: true}
		usecase := api.UsecasesWithCreds(r).NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.ListScenarioPublications(models.ListScenarioPublicationsFilters{
			ScenarioID:          utils.PtrTo(input.ScenarioID, options),
			ScenarioIterationID: utils.PtrTo(input.ScenarioIterationID, options),
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
		scenarioPublications, err := usecase.ExecuteScenarioPublicationAction(ctx, models.PublishScenarioIterationInput{
			ScenarioIterationId: input.Body.ScenarioIterationID,
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
