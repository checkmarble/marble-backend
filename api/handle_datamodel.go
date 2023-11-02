package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

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

func (d *DataModelHandler) GetDataModel(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	dataModel, err := d.useCase.GetDataModel(c.Request.Context(), organizationID)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data_model": dto.AdaptDataModelDto(dataModel),
	})
}

type createTableInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (d *DataModelHandler) CreateTable(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	var input createTableInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	tableID, err := d.useCase.CreateTable(c.Request.Context(), organizationID, input.Name, input.Description)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": tableID,
	})
}

func (d *DataModelHandler) UpdateDataModelTable(c *gin.Context) {
	var input createFieldInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}
	tableID := c.Param("tableID")

	err := d.useCase.UpdateDataModelTable(c.Request.Context(), tableID, input.Description)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

type createFieldInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Nullable    bool   `json:"nullable"`
	IsEnum      bool   `json:"is_enum"`
}

func (d *DataModelHandler) CreateField(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	var input createFieldInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
		return
	}

	tableID := c.Param("tableID")
	field := models.DataModelField{
		Name:        input.Name,
		Description: input.Description,
		Type:        input.Type,
		Nullable:    input.Nullable,
		IsEnum:      input.IsEnum,
	}

	fieldID, err := d.useCase.CreateField(c.Request.Context(), organizationID, tableID, field)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": fieldID,
	})
}

type updateFieldInput struct {
	Description *string `json:"description"`
	IsEnum      *bool   `json:"is_enum"`
}

func (d *DataModelHandler) UpdateDataModelField(c *gin.Context) {
	var input updateFieldInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	fieldID := c.Param("fieldID")

	err := d.useCase.UpdateDataModelField(c.Request.Context(), fieldID, models.UpdateDataModelFieldInput{
		Description: input.Description,
		IsEnum:      input.IsEnum,
	})
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

type createLinkInput struct {
	Name          string `json:"name"`
	ParentTableID string `json:"parent_table_id"`
	ParentFieldID string `json:"parent_field_id"`
	ChildTableID  string `json:"child_table_id"`
	ChildFieldID  string `json:"child_field_id"`
}

func (d *DataModelHandler) CreateLink(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	var input createLinkInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		http.Error(c.Writer, err.Error(), http.StatusBadRequest)
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

	err = d.useCase.CreateDataModelLink(c.Request.Context(), link)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (d *DataModelHandler) DeleteDataModel(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	err = d.useCase.DeleteDataModel(c.Request.Context(), organizationID)
	if presentError(c.Writer, c.Request, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (d *DataModelHandler) OpenAPI(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	dataModel, err := d.useCase.GetDataModel(c.Request.Context(), organizationID)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	openapi := dto.OpenAPIFromDataModel(dataModel)
	c.JSON(http.StatusOK, openapi)
}

func NewDataModelHandler(u dataModelUseCase) *DataModelHandler {
	return &DataModelHandler{
		useCase: u,
	}
}
