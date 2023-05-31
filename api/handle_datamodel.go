package api

import (
	"marble/marble-backend/utils"
	"net/http"
)

func (api *API) handleGetDataModel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.usecases.NewOrganizationUseCase()
		dataModel, err := usecase.GetDataModel(ctx, organizationId)
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "data_model", dataModel)
	}
}
