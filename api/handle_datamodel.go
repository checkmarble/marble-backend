package api

import (
	"net/http"

	"github.com/ggicci/httpin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleGetDataModel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		organizationID, err := api.UsecasesWithCreds(r).OrganizationIdOfContext()
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		dataModel, err := usecase.GetDataModel(organizationID)
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "data_model", dto.AdaptDataModelDto(dataModel))
	}
}

func (api *API) handleGetDataModelV2(w http.ResponseWriter, r *http.Request) {
	organizationID, err := api.UsecasesWithCreds(r).OrganizationIdOfContext()
	if presentError(w, r, err) {
		return
	}

	usecase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	dataModel, err := usecase.GetDataModel(organizationID)
	if presentError(w, r, err) {
		return
	}
	PresentModelWithName(w, "data_model", dto.AdaptDataModelDto(dataModel))
}

func (api *API) handlePostDataModel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		input := *ctx.Value(httpin.Input).(*dto.PostDataModel)

		organizationID, err := utils.OrgIDFromCtx(ctx, r)
		if presentError(w, r, err) {
			return
		}

		usecase := api.UsecasesWithCreds(r).NewOrganizationUseCase()
		dataModel, err := usecase.ReplaceDataModel(organizationID, dto.AdaptDataModel(input.Body.DataModel))
		if presentError(w, r, err) {
			return
		}

		PresentModelWithName(w, "data_model", dto.AdaptDataModelDto(dataModel))
	}
}

func (api *API) handleCreateTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input := *ctx.Value(httpin.Input).(*dto.PostCreateTable)

	organizationID, err := api.UsecasesWithCreds(r).OrganizationIdOfContext()
	if presentError(w, r, err) {
		return
	}

	usecase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	err = usecase.CreateDataModelTable(organizationID, input.Body.Name, input.Body.Description)
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

func (api *API) handleUpdateField(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input := *ctx.Value(httpin.Input).(*dto.PostCreateField)

	tableID, err := requiredUuidUrlParam(r, "fieldID")
	if presentError(w, r, err) {
		return
	}

	usecase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	err = usecase.UpdateDataModelField(tableID, input.Body.Description)
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}

func (api *API) handleCreateLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input := *ctx.Value(httpin.Input).(*dto.PostCreateLink)

	organizationID, err := api.UsecasesWithCreds(r).OrganizationIdOfContext()
	if presentError(w, r, err) {
		return
	}

	usecase := api.UsecasesWithCreds(r).NewDataModelUseCase()
	err = usecase.CreateDataModelLink(models.DataModelLink{
		Name:           models.LinkName(input.Body.Name),
		OrganizationID: organizationID,
		ParentTableID:  models.TableName(input.Body.ParentTableID),
		ParentFieldID:  models.FieldName(input.Body.ParentFieldID),
		ChildTableID:   models.TableName(input.Body.ChildTableID),
		ChildFieldID:   models.FieldName(input.Body.ChildFieldID),
	})
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}
