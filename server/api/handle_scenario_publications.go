package api

import (
	"encoding/json"
	"errors"
	"marble/marble-backend/models"
	"marble/marble-backend/server/dto"
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
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if utils.PresentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.ListScenarioPublicationsInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioID", input.ScenarioID))

		options := &utils.PtrToOptions{OmitZero: true}
		usecase := api.usecases.NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.ListScenarioPublications(ctx, orgID, models.ListScenarioPublicationsFilters{
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

func (api *API) CreateScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := utils.OrgIDFromCtx(ctx, r)
		if utils.PresentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*dto.CreateScenarioPublicationInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioIterationID", input.Body.ScenarioIterationID))

		if errors.Is(err, models.NotFoundError) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else if err != nil {
			logger.ErrorCtx(ctx, "Error getting scenario: \n"+err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		usecase := api.usecases.NewScenarioPublicationUsecase()
		scenarioPublications, err := usecase.CreateScenarioPublication(ctx, orgID, models.CreateScenarioPublicationInput{
			ScenarioIterationID: input.Body.ScenarioIterationID,
			PublicationAction:   models.PublicationActionFrom(input.Body.PublicationAction),
		})
		if errors.Is(err, models.ErrScenarioIterationNotValid) {
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
		if utils.PresentError(w, r, err) {
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioPublicationInput)
		logger := api.logger.With(slog.String("orgID", orgID), slog.String("scenarioPublicationID", input.ScenarioPublicationID))

		usecase := api.usecases.NewScenarioPublicationUsecase()
		scenarioPublication, err := usecase.GetScenarioPublication(ctx, orgID, input.ScenarioPublicationID)
		if errors.Is(err, models.NotFoundInRepositoryError) {
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
