package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"

	"github.com/ggicci/httpin"
)

func (api *API) handleGetOrganizations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		usecase := api.usecases.NewOrganizationUseCase()
		organizations, err := usecase.GetOrganizations(ctx)
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "organizations", utils.Map(organizations, dto.AdaptOrganizationDto))
	}
}

func (api *API) handlePostOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		inputDto := ctx.Value(httpin.Input).(*dto.CreateOrganizationInputDto).Body

		usecase := api.usecases.NewOrganizationUseCase()
		organization, err := usecase.CreateOrganization(ctx, models.CreateOrganizationInput{
			Name:         inputDto.Name,
			DatabaseName: inputDto.DatabaseName,
		})
		if presentError(w, r, err) {
			return
		}
		PresentModelWithName(w, "organization", dto.AdaptOrganizationDto(organization))
	}
}

func requiredOrgIdUrlParam(r *http.Request) (string, error) {
	return requiredUuidUrlParam(r, "orgID")
}

func (api *API) handleGetOrganizationUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := requiredOrgIdUrlParam(r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewOrganizationUseCase()
		users, err := usecase.GetUsersOfOrganization(orgID)
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "users", utils.Map(users, dto.AdaptUserDto))
	}
}

func (api *API) handleGetOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID, err := requiredOrgIdUrlParam(r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewOrganizationUseCase()
		organization, err := usecase.GetOrganization(ctx, orgID)

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "organization", dto.AdaptOrganizationDto(organization))
	}
}

func (api *API) handlePatchOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		input := ctx.Value(httpin.Input).(*dto.UpdateOrganizationInputDto)
		requestData := input.Body
		orgID := input.OrgID

		usecase := api.usecases.NewOrganizationUseCase()
		organization, err := usecase.UpdateOrganization(ctx, models.UpdateOrganizationInput{
			ID:                         orgID,
			Name:                       requestData.Name,
			DatabaseName:               requestData.DatabaseName,
			ExportScheduledExecutionS3: requestData.ExportScheduledExecutionS3,
		})

		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "organization", dto.AdaptOrganizationDto(organization))
	}
}

func (api *API) handleDeleteOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		orgID := ctx.Value(httpin.Input).(*dto.DeleteOrganizationInput).OrgID

		usecase := api.usecases.NewOrganizationUseCase()
		err := usecase.DeleteOrganization(ctx, orgID)
		if presentError(w, r, err) {
			return
		}
		PresentNothing(w)
	}
}
