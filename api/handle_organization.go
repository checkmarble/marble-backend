package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type OrganizationAppInterface interface {
	GetOrganizations(ctx context.Context) ([]app.Organization, error)
	CreateOrganization(ctx context.Context, organization app.CreateOrganizationInput) (app.Organization, error)

	GetOrganization(ctx context.Context, organizationID string) (app.Organization, error)
	UpdateOrganization(ctx context.Context, organization app.UpdateOrganizationInput) (app.Organization, error)
	// DeleteOrganization(ctx context.Context, organizationID string) error
}

type APIOrganization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (a *API) handleGetOrganizations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizations, err := a.app.GetOrganizations(ctx)
		if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting organizations: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		apiOrganizations := make([]APIOrganization, len(organizations))
		for i, org := range organizations {
			apiOrganizations[i] = APIOrganization{ID: org.ID, Name: org.Name}
		}

		err = json.NewEncoder(w).Encode(&apiOrganizations)
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type CreateOrganizationInput struct {
	Name         string `json:"name"`
	DatabaseName string `json:"databaseName"`
}

func (a *API) handlePostOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		requestData := &CreateOrganizationInput{}
		err := json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			// Could not parse JSON
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusBadRequest)
			return
		}

		org, err := a.app.CreateOrganization(ctx, app.CreateOrganizationInput{
			Name:         requestData.Name,
			DatabaseName: requestData.DatabaseName,
		})
		if err != nil {
			http.Error(w, fmt.Errorf("error creating organization: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(APIOrganization{ID: org.ID, Name: org.Name})
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (a *API) handleGetOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID := chi.URLParam(r, "orgID")

		org, err := a.app.GetOrganization(ctx, orgID)
		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting org(id: %s): %w", orgID, err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(APIOrganization{ID: org.ID, Name: org.Name})
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}

type UpdateOrganizationInput struct {
	Name         *string `json:"name"`
	DatabaseName *string `json:"databaseName"`
}

func (a *API) handlePutOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID := chi.URLParam(r, "orgID")

		requestData := &UpdateOrganizationInput{}
		err := json.NewDecoder(r.Body).Decode(requestData)
		if err != nil {
			// Could not parse JSON
			http.Error(w, fmt.Errorf("could not parse input JSON: %w", err).Error(), http.StatusBadRequest)
			return
		}

		org, err := a.app.UpdateOrganization(ctx, app.UpdateOrganizationInput{
			ID:           orgID,
			Name:         requestData.Name,
			DatabaseName: requestData.DatabaseName,
		})
		if errors.Is(err, app.ErrNotFoundInRepository) {
			http.Error(w, "", http.StatusNotFound)
			return
		} else if err != nil {
			// Could not execute request
			http.Error(w, fmt.Errorf("error getting org(id: %s): %w", orgID, err).Error(), http.StatusInternalServerError)
			return
		}

		err = json.NewEncoder(w).Encode(APIOrganization{ID: org.ID, Name: org.Name})
		if err != nil {
			// Could not encode JSON
			http.Error(w, fmt.Errorf("could not encode response JSON: %w", err).Error(), http.StatusInternalServerError)
			return
		}
	}
}
