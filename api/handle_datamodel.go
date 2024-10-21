package api

import (
	"encoding/json"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleGetDataModel(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		dataModel, err := usecase.GetDataModel(c.Request.Context(), organizationID)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"data_model": dto.AdaptDataModelDto(dataModel),
		})
	}
}

func handleCreateTable(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreateTableInput
		if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		tableID, err := usecase.CreateDataModelTable(c.Request.Context(), organizationID, input.Name, input.Description)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id": tableID,
		})
	}
}

func handleCreateField(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input dto.CreateFieldInput
		if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		tableID := c.Param("tableID")
		field := models.CreateFieldInput{
			TableId:     tableID,
			Name:        input.Name,
			Description: input.Description,
			DataType:    models.DataTypeFrom(input.Type),
			Nullable:    input.Nullable,
			IsEnum:      input.IsEnum,
			IsUnique:    input.IsUnique,
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		fieldID, err := usecase.CreateDataModelField(c.Request.Context(), field)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"id": fieldID,
		})
	}
}

func handleUpdateDataModelField(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input dto.UpdateFieldInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		fieldID := c.Param("fieldID")

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		err := usecase.UpdateDataModelField(c.Request.Context(), fieldID, models.UpdateFieldInput{
			Description: input.Description,
			IsEnum:      input.IsEnum,
			IsUnique:    input.IsUnique,
		})
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleCreateLink(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreateLinkInput
		if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		link := models.DataModelLinkCreateInput{
			OrganizationID: organizationID,
			Name:           input.Name,
			ParentTableID:  input.ParentTableId,
			ParentFieldID:  input.ParentFieldId,
			ChildTableID:   input.ChildTableId,
			ChildFieldID:   input.ChildFieldId,
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		_, err = usecase.CreateDataModelLink(c.Request.Context(), link)
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleDeleteDataModel(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		err = usecase.DeleteDataModel(c.Request.Context(), organizationID)
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleGetOpenAPI(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		dataModel, err := usecase.GetDataModel(c.Request.Context(), organizationID)
		if presentError(ctx, c, err) {
			return
		}

		openapi := dto.OpenAPIFromDataModel(dataModel)
		c.JSON(http.StatusOK, openapi)
	}
}
