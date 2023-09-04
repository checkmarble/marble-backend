package api

import (
	"marble/marble-backend/dto"
	"marble/marble-backend/utils"
	"net/http"
)

func (api *API) handleGetApiKey() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		organizationId, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}
		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		apiKeys, err := usecase.GetApiKeysOfOrganization(ctx, organizationId)
		if presentError(w, r, err) {
			return
		}

		apiKeysDto := utils.Map(apiKeys, dto.AdaptApiKeyDto)
		PresentModelWithName(w, "api_keys", apiKeysDto)
	}
}
