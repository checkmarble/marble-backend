package api

import (
	"encoding/json"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) GetDataModel(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	dataModel, err := usecase.GetDataModel(c.Request.Context(), organizationID)
	if presentError(c, err) {
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

func (api *API) CreateTable(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	var input createTableInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	tableID, err := usecase.CreateDataModelTable(c.Request.Context(), organizationID, input.Name, input.Description)
	if presentError(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id": tableID,
	})
}

func (api *API) UpdateDataModelTable(c *gin.Context) {
	var input createFieldInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}
	tableID := c.Param("tableID")

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	err := usecase.UpdateDataModelTable(c.Request.Context(), tableID, input.Description)
	if presentError(c, err) {
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

func (api *API) CreateField(c *gin.Context) {
	var input createFieldInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	tableID := c.Param("tableID")
	field := models.CreateFieldInput{
		TableId:     tableID,
		Name:        models.FieldName(input.Name),
		Description: input.Description,
		DataType:    models.DataTypeFrom(input.Type),
		Nullable:    input.Nullable,
		IsEnum:      input.IsEnum,
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	fieldID, err := usecase.CreateDataModelField(c.Request.Context(), field)
	if presentError(c, err) {
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

func (api *API) UpdateDataModelField(c *gin.Context) {
	var input updateFieldInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	fieldID := c.Param("fieldID")

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	err := usecase.UpdateDataModelField(c.Request.Context(), fieldID, models.UpdateFieldInput{
		Description: input.Description,
		IsEnum:      input.IsEnum,
	})
	if presentError(c, err) {
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

func (api *API) CreateLink(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	var input createLinkInput
	if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	link := models.DataModelLinkCreateInput{
		OrganizationID: organizationID,
		Name:           models.LinkName(input.Name),
		ParentTableID:  input.ParentTableID,
		ParentFieldID:  input.ParentFieldID,
		ChildTableID:   input.ChildTableID,
		ChildFieldID:   input.ChildFieldID,
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	err = usecase.CreateDataModelLink(c.Request.Context(), link)
	if presentError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *API) DeleteDataModel(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	err = usecase.DeleteDataModel(c.Request.Context(), organizationID)
	if presentError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (api *API) OpenAPI(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewDataModelUseCase()
	dataModel, err := usecase.GetDataModel(c.Request.Context(), organizationID)
	if presentError(c, err) {
		return
	}

	openapi := dto.OpenAPIFromDataModel(dataModel)
	c.JSON(http.StatusOK, openapi)
}
