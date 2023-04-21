package api

import (
	"context"
	"encoding/json"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
)

type ScenarioPublicationAppInterface interface {
	ReadScenarioPublications(ctx context.Context, orgID string, filters app.ReadScenarioPublicationsFilters) ([]app.ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, orgID string, sp app.CreateScenarioPublicationInput) ([]app.ScenarioPublication, error)
}

type APIScenarioPublication struct {
	ID   string `json:"id"`
	Rank int32  `json:"rank"`
	// UserID              string    `json:"userID"`
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

type GetScenarioPublicationsInput struct {
	ID string `in:"query=id"`
	// UserID              string `in:"query=userID"`
	ScenarioID          string `in:"query=scenarioID"`
	ScenarioIterationID string `in:"query=scenarioIterationID"`
	PublicationAction   string `in:"query=publicationAction"`
}

func (api *API) handleGetScenarioPublications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioPublicationsInput)

		options := &utils.PtrToOptions{OmitZero: true}
		scenarioPublications, err := api.app.ReadScenarioPublications(ctx, orgID, app.ReadScenarioPublicationsFilters{
			ID:         utils.PtrTo(input.ID, options),
			ScenarioID: utils.PtrTo(input.ScenarioID, options),
			// UserID:              utils.PtrTo(input.UserID,, options),
			ScenarioIterationID: utils.PtrTo(input.ScenarioIterationID, options),
			PublicationAction:   utils.PtrTo(input.PublicationAction, options),
		})
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting scenario publications: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		scenarioPublicationDTOs := make([]APIScenarioPublication, len(scenarioPublications))
		for i, sp := range scenarioPublications {
			scenarioPublicationDTOs[i] = NewAPIScenarioPublication(sp)
		}

		err = json.NewEncoder(w).Encode(scenarioPublicationDTOs)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type PostScenarioPublicationBody struct {
	// UserID              string    `json:"userID"`
	ScenarioID          string `json:"scenarioID"`
	ScenarioIterationID string `json:"scenarioIterationID"`
	PublicationAction   string `json:"publicationAction"`
}

type PostScenarioPublicationInput struct {
	Body *PostScenarioPublicationBody `in:"body=json"`
}

func (api *API) handlePostScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*PostScenarioPublicationInput)

		scenarioPublications, err := api.app.CreateScenarioPublication(ctx, orgID, app.CreateScenarioPublicationInput{
			// UserID: input.Body.UserID,
			ScenarioID:          input.Body.ScenarioID,
			ScenarioIterationID: input.Body.ScenarioIterationID,
			PublicationAction:   app.PublicationActionFrom(input.Body.PublicationAction),
		})
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error handling scenario publication: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		scenarioPublicationDTOs := make([]APIScenarioPublication, len(scenarioPublications))
		for i, sp := range scenarioPublications {
			scenarioPublicationDTOs[i] = NewAPIScenarioPublication(sp)
		}

		err = json.NewEncoder(w).Encode(scenarioPublicationDTOs)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
