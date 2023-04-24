package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/utils"
	"net/http"
	"time"

	"github.com/ggicci/httpin"
)

type ScenarioPublicationAppInterface interface {
	ListScenarioPublications(ctx context.Context, orgID string, filters app.ListScenarioPublicationsFilters) ([]app.ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, orgID string, sp app.CreateScenarioPublicationInput) ([]app.ScenarioPublication, error)
	GetScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (app.ScenarioPublication, error)
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

type ListScenarioPublicationsInput struct {
	// UserID              string `in:"query=userID"`
	ScenarioID          string `in:"query=scenarioID"`
	ScenarioIterationID string `in:"query=scenarioIterationID"`
	PublicationAction   string `in:"query=publicationAction"`
}

func (api *API) ListScenarioPublications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*ListScenarioPublicationsInput)

		options := &utils.PtrToOptions{OmitZero: true}
		scenarioPublications, err := api.app.ListScenarioPublications(ctx, orgID, app.ListScenarioPublicationsFilters{
			// UserID:              utils.PtrTo(input.UserID,, options),
			ScenarioID:          utils.PtrTo(input.ScenarioID, options),
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

type CreateScenarioPublicationBody struct {
	// UserID              string    `json:"userID"`
	ScenarioID          string `json:"scenarioID"`
	ScenarioIterationID string `json:"scenarioIterationID"`
	PublicationAction   string `json:"publicationAction"`
}

type CreateScenarioPublicationInput struct {
	Body *CreateScenarioPublicationBody `in:"body=json"`
}

func (api *API) CreateScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*CreateScenarioPublicationInput)

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

type GetScenarioPublicationInput struct {
	ScenarioPublicationID string `in:"path=scenarioPublicationID"`
}

func (api *API) GetScenarioPublication() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := orgIDFromCtx(ctx)
		if err != nil {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		input := ctx.Value(httpin.Input).(*GetScenarioPublicationInput)

		scenarioPublication, err := api.app.GetScenarioPublication(ctx, orgID, input.ScenarioPublicationID)
		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			// TODO(errors): handle missing fields error ?
			http.Error(w, fmt.Errorf("error getting scenario publication(id: %s): %w", input.ScenarioPublicationID, err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(NewAPIScenarioPublication(scenarioPublication))
		if err != nil {
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
