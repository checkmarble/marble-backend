package api

import (
	"context"
	"encoding/json"
	"fmt"
	"marble/marble-backend/app"
	"net/http"
	"time"
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

type ReadScenarioPublicationsFilters struct {
	ID         *string
	ScenarioID *string
	// UserID              *string
	ScenarioIterationID *string
	PublicationAction   *string
}

func (api *API) handleGetScenarioPublications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		//TODO(filters): get filters from query parameters
		requestData := &ReadScenarioPublicationsFilters{}
		// err = json.NewDecoder(r.Body).Decode(requestData)
		// if err != nil {
		// 	http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
		// 	return
		// }

		scenarioPublications, err := api.app.ReadScenarioPublications(ctx, orgID, app.ReadScenarioPublicationsFilters{
			ID:         requestData.ID,
			ScenarioID: requestData.ScenarioID,
			// UserID:              requestData.UserID,
			ScenarioIterationID: requestData.ScenarioIterationID,
			PublicationAction:   requestData.PublicationAction,
		})
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting scenario publications: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		var scenarioPublicationDTOs []APIScenarioPublication
		for _, sp := range scenarioPublications {
			scenarioPublicationDTOs = append(scenarioPublicationDTOs, NewAPIScenarioPublication(sp))
		}

		err = json.NewEncoder(w).Encode(scenarioPublicationDTOs)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type CreateScenarioPublicationInput struct {
	// UserID              string    `json:"userID"`
	ScenarioID          string `json:"scenarioID"`
	ScenarioIterationID string `json:"scenarioIterationID"`
	PublicationAction   string `json:"publicationAction"`
}

func (api *API) handlePostScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		requestData := &CreateScenarioPublicationInput{}
		err = json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusUnprocessableEntity)
			return
		}

		scenarioPublications, err := api.app.CreateScenarioPublication(ctx, orgID, app.CreateScenarioPublicationInput{
			// UserID: requestData.UserID,
			ScenarioID:          requestData.ScenarioID,
			ScenarioIterationID: requestData.ScenarioIterationID,
			PublicationAction:   app.PublicationActionFrom(requestData.PublicationAction),
		})
		if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error handling scenario publication: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		var scenarioPublicationDTOs []APIScenarioPublication
		for _, sp := range scenarioPublications {
			scenarioPublicationDTOs = append(scenarioPublicationDTOs, NewAPIScenarioPublication(sp))
		}

		err = json.NewEncoder(w).Encode(scenarioPublicationDTOs)
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
