package api

import (
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

type APIOrganization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewAPIOrganization(org models.Organization) APIOrganization {
	return APIOrganization{
		ID:   org.ID,
		Name: org.Name,
	}
}

func (api *API) handleGetOrganizations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		usecase := api.usecases.NewOrganizationUseCase()
		organizations, err := usecase.GetOrganizations(ctx)
		if presentError(ctx, api.logger, w, err) {
			return
		}

		apiOrganizations := utils.Map(organizations, NewAPIOrganization)

		PresentModel(w, struct {
			Organizations []APIOrganization `json:"organizations"`
		}{
			Organizations: apiOrganizations,
		})
	}
}

type CreateOrganizationBodyDto struct {
	Name         string `json:"name"`
	DatabaseName string `json:"databaseName"`
}

type CreateOrganizationInputDto struct {
	Body *CreateOrganizationBodyDto `in:"body=json"`
}

func presentOrganization(w http.ResponseWriter, organization models.Organization) {
	PresentModel(w, struct {
		Organization APIOrganization `json:"organization"`
	}{
		Organization: NewAPIOrganization(organization),
	})
}

func (api *API) handlePostOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		inputDto := ctx.Value(httpin.Input).(*CreateOrganizationInputDto).Body

		usecase := api.usecases.NewOrganizationUseCase()
		organization, err := usecase.CreateOrganization(ctx, models.CreateOrganizationInput{
			Name:         inputDto.Name,
			DatabaseName: inputDto.DatabaseName,
		})
		if presentError(ctx, api.logger, w, err) {
			return
		}
		presentOrganization(w, organization)
	}
}

func (api *API) handleGetOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := requiredUuidUrlParam(r, "orgID")
		if presentError(ctx, api.logger, w, err) {
			return
		}

		usecase := api.usecases.NewOrganizationUseCase()
		organization, err := usecase.GetOrganization(ctx, orgID)

		if presentError(ctx, api.logger, w, err) {
			return
		}

		presentOrganization(w, organization)
	}
}

type UpdateOrganizationBodyDto struct {
	Name         *string `json:"name,omitempty"`
	DatabaseName *string `json:"databaseName,omitempty"`
}

type UpdateOrganizationInputDto struct {
	OrgID string                     `in:"path=orgID"`
	Body  *UpdateOrganizationBodyDto `in:"body=json"`
}

func (api *API) handlePutOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*UpdateOrganizationInputDto)
		requestData := input.Body
		orgID := input.OrgID

		usecase := api.usecases.NewOrganizationUseCase()
		organization, err := usecase.UpdateOrganization(ctx, models.UpdateOrganizationInput{
			ID:           orgID,
			Name:         requestData.Name,
			DatabaseName: requestData.DatabaseName,
		})

		if presentError(ctx, api.logger, w, err) {
			return
		}

		presentOrganization(w, organization)
	}
}

type DeleteOrganizationInput struct {
	orgID string `in:"path=orgID"`
}

func (api *API) handleDeleteOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID := ctx.Value(httpin.Input).(*DeleteOrganizationInput).orgID

		usecase := api.usecases.NewOrganizationUseCase()
		err := usecase.SoftDeleteOrganization(ctx, orgID)
		if presentError(ctx, api.logger, w, err) {
			return
		}
		PresentNothing(w)
	}
}
