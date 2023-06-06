package api

import (
	"encoding/json"
	"errors"
	"marble/marble-backend/app"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
	"golang.org/x/exp/slog"
)

type APIScenarioPublication struct {
	ID                  string    `json:"id"`
	Rank                int32     `json:"rank"`
	ScenarioID          string    `json:"scenarioID"`
	ScenarioIterationID string    `json:"scenarioIterationID"`
	PublicationAction   string    `json:"publicationAction"`
	CreatedAt           time.Time `json:"createdAt"`
}

func NewAPIScenarioPublication(sp app.ScenarioPublication) APIScenarioPublication {
	return APIScenarioPublication{
		ID:   sp.ID,
		Rank: sp.Rank,
		// UserID:              sp.UserID,
		ScenarioID:          sp.ScenarioID,
		ScenarioIterationID: sp.ScenarioIterationID,
		PublicationAction:   sp.PublicationAction.String(),
		CreatedAt:           sp.CreatedAt,
	}
}

type ListScenarioPublicationsInput struct {
	ScenarioID          string `in:"query=scenarioID"`
	ScenarioIterationID string `in:"query=scenarioIterationID"`
	PublicationAction   string `in:"query=publicationAction"`
}

func (api *API) ListScenarioPublications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*ListScenarioPublicationsInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioID", input.ScenarioID))

		options := &utils.PtrToOptions{OmitZero: true}
		usecase := api.usecases.NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.ListScenarioPublications(ctx, orgID, app.ListScenarioPublicationsFilters{
			ScenarioID:          utils.PtrTo(input.ScenarioID, options),
			ScenarioIterationID: utils.PtrTo(input.ScenarioIterationID, options),
			PublicationAction:   utils.PtrTo(input.PublicationAction, options),
		})
		if err != nil {
			logger.ErrorCtx(ctx, "Error listing scenario publications: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		scenarioPublicationDTOs := make([]APIScenarioPublication, len(scenarioPublications))
		for i, sp := range scenarioPublications {
			scenarioPublicationDTOs[i] = NewAPIScenarioPublication(sp)
		}

		err = json.NewEncoder(w).Encode(scenarioPublicationDTOs)
		if err != nil {
			logger.ErrorCtx(ctx, "Error encoding scenario publications: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

type CreateScenarioPublicationBody struct {
	ScenarioIterationID string `json:"scenarioIterationID"`
	PublicationAction   string `json:"publicationAction"`
}

type CreateScenarioPublicationInput struct {
	Body *CreateScenarioPublicationBody `in:"body=json"`
}

func (api *API) CreateScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*CreateScenarioPublicationInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioIterationID", input.Body.ScenarioIterationID))

		usecase := api.usecases.NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.CreateScenarioPublication(ctx, orgID, app.CreateScenarioPublicationInput{
			ScenarioIterationID: input.Body.ScenarioIterationID,
			PublicationAction:   app.PublicationActionFrom(input.Body.PublicationAction),
		})
		if errors.Is(err, app.ErrScenarioIterationNotValid) {
			logger.WarnCtx(ctx, "Scenario iteration not valid")
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Error creating scenario publication: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		scenarioPublicationDTOs := make([]APIScenarioPublication, len(scenarioPublications))
		for i, sp := range scenarioPublications {
			scenarioPublicationDTOs[i] = NewAPIScenarioPublication(sp)
		}

		err = json.NewEncoder(w).Encode(scenarioPublicationDTOs)
		if err != nil {
			logger.ErrorCtx(ctx, "Error encoding scenario publications: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}

type GetScenarioPublicationInput struct {
	ScenarioPublicationID string `in:"path=scenarioPublicationID"`
}

func (api *API) GetScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioPublicationInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioPublicationID", input.ScenarioPublicationID))

		usecase := api.usecases.NewScenarioPublicationUsecase()
		scenarioPublication, err := usecase.GetScenarioPublication(ctx, orgID, input.ScenarioPublicationID)
		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Error getting scenario publication: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIScenarioPublication(scenarioPublication))
		if err != nil {
			logger.ErrorCtx(ctx, "Error encoding scenario publication: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}
}
