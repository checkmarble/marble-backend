package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
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

	usecase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	err = usecase.CreateDataModelTable(organizationId, input.Body.Name, input.Body.Description)
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}

func (api *API) handleUpdateTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input := *ctx.Value(httpin.Input).(*dto.PostCreateTable)

	tableID, err := requiredUuidUrlParam(r, "tableID")
	if presentError(w, r, err) {
		return
	}

	usecase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	err = usecase.UpdateDataModelTable(tableID, input.Body.Description)
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}

func (api *API) handleCreateField(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input := *ctx.Value(httpin.Input).(*dto.PostCreateField)

	tableID, err := requiredUuidUrlParam(r, "tableID")
	if presentError(w, r, err) {
		return
	}

	usecase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	err = usecase.CreateDataModelField(tableID, models.DataModelField{
		Name:        input.Body.Name,
		Description: input.Body.Description,
		Type:        input.Body.Type,
		Nullable:    input.Body.Nullable,
	})
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}

func (api *API) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input := *ctx.Value(httpin.Input).(*dto.PostCreateLink)

	usecase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	err := usecase.CreateDataModelLink(models.DataModelLink{
		Name:          input.Body.Name,
		ParentTableID: input.Body.ParentTableID,
		ParentFieldID: input.Body.ParentFieldID,
		ChildTableID:  input.Body.ChildTableID,
		ChildFieldID:  input.Body.ChildFieldID,
	})
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}
