package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
)

func handleGetObjectMetadata(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		objectType := c.Param("object-type")
		if objectType == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "object-type is required"))
			return
		}

		objectId := c.Param("object-id")
		if objectId == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "object-id is required"))
			return
		}

		metadataTypeStr := c.Param("type")
		if metadataTypeStr == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "metadata-type is required"))
			return
		}

		metadataType := models.MetadataTypeFrom(metadataTypeStr)
		if metadataType == models.MetadataTypeUnknown {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid metadata-type"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectMetadataUsecase()
		objectMetadata, err := usecase.GetObjectMetadata(ctx, organizationId, objectType, objectId, metadataType)
		if presentError(ctx, c, err) {
			return
		}

		result, err := dto.AdaptObjectMetadataDto(objectMetadata)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

func handleUpsertObjectMetadata(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		objectType := c.Param("object-type")
		if objectType == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "object-type is required"))
			return
		}

		objectId := c.Param("object-id")
		if objectId == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "object-id is required"))
			return
		}

		metadataTypeStr := c.Param("type")
		if metadataTypeStr == "" {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "metadata-type is required"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectMetadataUsecase()
		var newObjectMetadata models.ObjectMetadata

		metadataType := models.MetadataTypeFrom(metadataTypeStr)
		switch metadataType {
		case models.MetadataTypeRiskTopics:
			var input dto.ObjectRiskTopicUpsertInputDto
			if err := c.ShouldBindJSON(&input); err != nil {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
				return
			}
			upsertInput, err := input.Adapt(organizationId, objectType, objectId)
			if presentError(ctx, c, err) {
				return
			}
			newObjectMetadata, err = usecase.UpsertObjectRiskTopic(ctx, upsertInput)
			if presentError(ctx, c, err) {
				return
			}
		default:
			presentError(ctx, c, errors.Wrap(models.BadParameterError,
				"upsert not supported for metadata-type: "+metadataTypeStr))
			return
		}

		result, err := dto.AdaptObjectMetadataDto(newObjectMetadata)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
