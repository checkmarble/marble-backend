package api

import (
	"net/http"

	"github.com/ggicci/httpin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleGetOrganizations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
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

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
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
	return requiredUuidUrlParam(r, "organizationId")
}

func (api *API) handleGetOrganizationUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationId, err := requiredOrgIdUrlParam(r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		users, err := usecase.GetUsersOfOrganization(organizationId)
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "users", utils.Map(users, dto.AdaptUserDto))
	}
}

func (api *API) handleGetOrganization() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := requiredOrgIdUrlParam(r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		organization, err := usecase.GetOrganization(ctx, organizationId)

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
		organizationId := input.OrganizationId

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		organization, err := usecase.UpdateOrganization(ctx, models.UpdateOrganizationInput{
			Id:                         organizationId,
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

		organizationId := ctx.Value(httpin.Input).(*dto.DeleteOrganizationInput).OrganizationId

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		err := usecase.DeleteOrganization(ctx, organizationId)
		if presentError(w, r, err) {
			return
		}
		PresentNothing(w)
	}
}
