package api

import (
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
	"net/http"
)

func (api *API) handleGetDataModel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(ctx, api.logger, w, err) {
			return
		}

		usecase := api.usecases.NewOrganizationUseCase()
		dataModel, err := usecase.GetDataModel(ctx, organizationId)
		if presentError(ctx, api.logger, w, err) {
			return
		}

		PresentModel(w, struct {
			DataModel models.DataModel `json:"data_model"`
		}{
			DataModel: dataModel,
		})
	}
}
