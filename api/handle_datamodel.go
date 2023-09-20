package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/ggicci/httpin"
)

func (api *API) handleGetDataModel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		dataModel, err := usecase.GetDataModel(organizationId)
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "data_model", dto.AdaptDataModelDto(dataModel))
	}
}

func (api *API) handlePostDataModel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		input := *ctx.Value(httpin.Input).(*dto.PostDataModel)

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		dataModel, err := usecase.ReplaceDataModel(organizationId, dto.AdaptDataModel(input.Body.DataModel))
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "data_model", dto.AdaptDataModelDto(dataModel))
	}
}

func (api *API) handleCreateTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input := *ctx.Value(httpin.Input).(*dto.PostCreateTable)

	organizationId, err := utils.OrgIDFromCtx(ctx, r)
	if presentError(w, r, err) {
		return
	}

	usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
	err = usecase.CreateDataModelTable(organizationId, input.Body.Name, input.Body.Description)
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}
