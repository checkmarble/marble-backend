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

type objectMetadataQueryParams struct {
	ObjectType   string `uri:"object-type" binding:"required"`
	ObjectID     string `uri:"object-id" binding:"required"`
	MetadataType string `uri:"type" binding:"required"`
}

func handleGetObjectMetadata(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var params objectMetadataQueryParams
		err = c.ShouldBindUri(&params)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		metadataType := models.MetadataTypeFrom(params.MetadataType)
		if metadataType == models.MetadataTypeUnknown {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid metadata-type"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectMetadataUsecase()
		objectMetadata, err := usecase.GetObjectMetadata(
			ctx,
			organizationId,
			params.ObjectType,
			params.ObjectID,
			metadataType,
		)
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

		var params objectMetadataQueryParams
		err = c.ShouldBindUri(&params)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewObjectMetadataUsecase()
		var newObjectMetadata models.ObjectMetadata
		metadataType := models.MetadataTypeFrom(params.MetadataType)
		switch metadataType {
		case models.MetadataTypeRiskTopics:
			var input dto.ObjectRiskTopicUpsertInputDto
			if err := c.ShouldBindJSON(&input); err != nil {
				presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
				return
			}
			upsertInput, err := input.Adapt(organizationId, params.ObjectType, params.ObjectID)
			if presentError(ctx, c, err) {
				return
			}
			newObjectMetadata, err = usecase.UpsertObjectRiskTopic(ctx, upsertInput)
			if presentError(ctx, c, err) {
				return
			}
		default:
			presentError(
				ctx,
				c,
				errors.Wrap(models.BadParameterError, "invalid metadata-type"),
			)
			return
		}

		result, err := dto.AdaptObjectMetadataDto(newObjectMetadata)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
