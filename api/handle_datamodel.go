package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/utils"
	"net/http"

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
