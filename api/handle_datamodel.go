package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type dataModelUseCase interface {
	GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error)
	CreateTable(ctx context.Context, organizationID, name, description string) (string, error)
	UpdateDataModelTable(ctx context.Context, tableID, description string) error
	CreateField(ctx context.Context, organizationID, tableID string, field models.DataModelField) (string, error)
	UpdateDataModelField(ctx context.Context, fieldID string, input models.UpdateDataModelFieldInput) error
	DeleteDataModel(ctx context.Context, organizationID string) error
	CreateDataModelLink(ctx context.Context, link models.DataModelLink) error
}

type DataModelHandler struct {
	useCase dataModelUseCase
}

func (d *DataModelHandler) GetDataModel(w http.ResponseWriter, r *http.Request) {
	organizationID, err := utils.OrganizationIdFromRequest(r)
	if presentError(w, r, err) {
		return
	}

	dataModel, err := d.useCase.GetDataModel(r.Context(), organizationID)
	if presentError(w, r, err) {
		return
	}
	PresentModelWithName(w, "data_model", dto.AdaptDataModelDto(dataModel))
}

type createTableInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (d *DataModelHandler) CreateTable(w http.ResponseWriter, r *http.Request) {
	organizationID, err := utils.OrganizationIdFromRequest(r)
	if presentError(w, r, err) {
		return
	}

	var input createTableInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tableID, err := d.useCase.CreateTable(r.Context(), organizationID, input.Name, input.Description)
	if presentError(w, r, err) {
		return
	}
	PresentModelWithName(w, "id", tableID)
}

func (d *DataModelHandler) UpdateDataModelTable(w http.ResponseWriter, r *http.Request) {
	var input createFieldInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tableID := chi.URLParam(r, "tableID")

	err := d.useCase.UpdateDataModelTable(r.Context(), tableID, input.Description)
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}

type createFieldInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Nullable    bool   `json:"nullable"`
	IsEnum      bool   `json:"is_enum"`
}

type updateFieldInput struct {
	Description *string `json:"description"`
	IsEnum      *bool   `json:"is_enum"`
}

func (d *DataModelHandler) CreateField(w http.ResponseWriter, r *http.Request) {
	organizationID, err := utils.OrganizationIdFromRequest(r)
	if presentError(w, r, err) {
		return
	}

	var input createFieldInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tableID := chi.URLParam(r, "tableID")
	field := models.DataModelField{
		Name:        input.Name,
		Description: input.Description,
		Type:        input.Type,
		Nullable:    input.Nullable,
		IsEnum:      input.IsEnum,
	}

	fieldID, err := d.useCase.CreateField(r.Context(), organizationID, tableID, field)
	if presentError(w, r, err) {
		return
	}
	PresentModelWithName(w, "id", fieldID)
}

func (d *DataModelHandler) UpdateDataModelField(w http.ResponseWriter, r *http.Request) {
	var input updateFieldInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fieldID := chi.URLParam(r, "fieldID")

	err := d.useCase.UpdateDataModelField(r.Context(), fieldID, models.UpdateDataModelFieldInput{
		Description: input.Description,
		IsEnum:      input.IsEnum,
	})
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}

type createLinkInput struct {
	Name          string `json:"name"`
	ParentTableID string `json:"parent_table_id"`
	ParentFieldID string `json:"parent_field_id"`
	ChildTableID  string `json:"child_table_id"`
	ChildFieldID  string `json:"child_field_id"`
}

func (d *DataModelHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	organizationID, err := utils.OrganizationIdFromRequest(r)
	if presentError(w, r, err) {
		return
	}

	var input createLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	link := models.DataModelLink{
		OrganizationID: organizationID,
		Name:           models.LinkName(input.Name),
		ParentTableID:  input.ParentTableID,
		ParentFieldID:  input.ParentFieldID,
		ChildTableID:   input.ChildTableID,
		ChildFieldID:   input.ChildFieldID,
	}

	err = d.useCase.CreateDataModelLink(r.Context(), link)
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}

func (d *DataModelHandler) DeleteDataModel(w http.ResponseWriter, r *http.Request) {
	organizationID, err := utils.OrganizationIdFromRequest(r)
	if presentError(w, r, err) {
		return
	}

	err = d.useCase.DeleteDataModel(r.Context(), organizationID)
	if presentError(w, r, err) {
		return
	}
	PresentNothing(w)
}

func (d *DataModelHandler) OpenAPI(w http.ResponseWriter, r *http.Request) {
	organizationID, err := utils.OrganizationIdFromRequest(r)
	if presentError(w, r, err) {
		return
	}

	dataModel, err := d.useCase.GetDataModel(r.Context(), organizationID)
	if presentError(w, r, err) {
		return
	}

	openapi := dto.OpenAPIFromDataModel(dataModel)
	PresentModel(w, openapi)
}

func NewDataModelHandler(u dataModelUseCase) *DataModelHandler {
	return &DataModelHandler{
		useCase: u,
	}
}
