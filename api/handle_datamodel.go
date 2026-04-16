package api

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pure_utils"
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
		dataModel, err := usecase.GetDataModel(ctx, organizationID, models.DataModelReadOptions{
			IncludeEnums:              true,
			IncludeNavigationOptions:  true,
			IncludeUnicityConstraints: true,
		}, false)
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

		modelInput, err := input.AdaptToModel()
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		tableID, err := usecase.CreateDataModelTable(ctx, organizationID, modelInput)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{
			"id": tableID,
		})
	}
}

func handleUpdateDataModelTable(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var input dto.UpdateTableInput
		if err := json.NewDecoder(c.Request.Body).Decode(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		tableID := c.Param("tableID")

		modelInput, err := input.AdaptToUpdateTableCompositeInput()
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		report, err := usecase.UpdateDataModelTableComposite(ctx, tableID, modelInput)
		if errors.Is(err, models.ConflictError) {
			c.JSON(http.StatusConflict, dto.AdaptDataModelDeleteFieldReport(report, err))
			return
		}
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
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

		var ftmProperty *models.FollowTheMoneyProperty
		if input.FTMProperty != nil {
			property := models.FollowTheMoneyPropertyFrom(*input.FTMProperty)
			if property == models.FollowTheMoneyPropertyUnknown {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid FTM property"))
				return
			}
			ftmProperty = &property
		}

		var fieldSemanticType models.FieldSemanticType
		if input.SemanticType != nil {
			fieldSemanticType = models.FieldSemanticType(*input.SemanticType)
			if !fieldSemanticType.IsValid() {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid field semantic type"))
				return
			}
		}

		tableID := c.Param("tableID")
		field := models.CreateFieldInput{
			TableId:      tableID,
			Name:         input.Name,
			Description:  input.Description,
			Alias:        input.Alias,
			SemanticType: fieldSemanticType,
			DataType:     models.DataTypeFrom(input.Type),
			Nullable:     input.Nullable,
			IsEnum:       input.IsEnum,
			IsUnique:     input.IsUnique,
			FTMProperty:  ftmProperty,
			Metadata:     input.Metadata,
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		fieldID, err := usecase.CreateDataModelField(ctx, field)
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

		var ftmProperty pure_utils.Null[models.FollowTheMoneyProperty]
		if input.FTMProperty.Set {
			if input.FTMProperty.Valid {
				property := models.FollowTheMoneyPropertyFrom(input.FTMProperty.Value())
				if property == models.FollowTheMoneyPropertyUnknown {
					presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid FTM property"))
					return
				}
				ftmProperty = pure_utils.NullFrom(property)
			} else {
				ftmProperty = pure_utils.NullFromPtr[models.FollowTheMoneyProperty](nil)
			}
		}

		var fieldSemanticType pure_utils.Null[models.FieldSemanticType]
		if input.SemanticType.Set {
			if input.SemanticType.Valid {
				semanticType := models.FieldSemanticType(input.SemanticType.Value())
				if !semanticType.IsValid() {
					presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid field semantic type"))
					return
				}
				fieldSemanticType = pure_utils.NullFrom(semanticType)
			} else {
				fieldSemanticType = pure_utils.NullFromPtr[models.FieldSemanticType](nil)
			}
		}

		if input.Alias != nil && *input.Alias == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "field alias cannot be empty"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		err := usecase.UpdateDataModelField(ctx, fieldID, models.UpdateFieldInput{
			Description:  input.Description,
			IsEnum:       input.IsEnum,
			IsUnique:     input.IsUnique,
			IsNullable:   input.IsNullable,
			FTMProperty:  ftmProperty,
			Alias:        input.Alias,
			SemanticType: fieldSemanticType,
			Metadata:     input.Metadata,
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

		link, err := input.AdaptToModel(organizationID)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		_, err = usecase.CreateDataModelLink(ctx, link)
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
		err = usecase.DeleteDataModel(ctx, organizationID)
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func handleGetOpenAPI(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		if !regexp.MustCompile(`^v[0-9\.]+$`).Match([]byte(c.Param("version"))) {
			presentError(ctx, c, errors.Wrap(models.NotFoundError, "invalid version string"))
			return
		}

		organizationID, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		openapi, err := pubapi.GetOpenApiForVersion(c.Param("version"))
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		dataModel, err := usecase.GetDataModel(ctx, organizationID, models.DataModelReadOptions{
			IncludeEnums:              false,
			IncludeNavigationOptions:  false,
			IncludeUnicityConstraints: false,
		}, false)
		if presentError(ctx, c, err) {
			return
		}

		spec, err := dto.OpenAPIFromDataModel(dataModel, openapi)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, spec)
	}
}

func handleCreateNavigationOption(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		sourceTableId := c.Param("tableID")

		var input dto.CreateNavigationOptionInput
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewDataModelUseCase()
		err := usecase.CreateNavigationOption(ctx, models.CreateNavigationOptionInput{
			SourceTableId:   sourceTableId,
			SourceFieldId:   input.SourceFieldId,
			TargetTableId:   input.TargetTableId,
			FilterFieldId:   input.FilterFieldId,
			OrderingFieldId: input.OrderingFieldId,
		})
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
