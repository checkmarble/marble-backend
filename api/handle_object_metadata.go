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
		if metadataType != models.MetadataTypeRiskTopics {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid metadata-type"))
			return
		}

		// Fetch risk topic annotation from entity_annotations table
		usecase := usecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()
		riskTopicType := models.EntityAnnotationRiskTopic
		annotations, err := usecase.List(ctx, models.EntityAnnotationRequest{
			OrgId:          organizationId,
			ObjectType:     params.ObjectType,
			ObjectId:       params.ObjectID,
			AnnotationType: &riskTopicType,
		})
		if presentError(ctx, c, err) {
			return
		}

		if len(annotations) == 0 {
			presentError(ctx, c, models.NotFoundError)
			return
		}

		// Return first risk topic annotation (there should only be one per object)
		result, err := dto.AdaptRiskTopicAnnotationToObjectMetadataDto(annotations[0])
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

		metadataType := models.MetadataTypeFrom(params.MetadataType)
		if metadataType != models.MetadataTypeRiskTopics {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid metadata-type"))
			return
		}

		var input dto.ObjectRiskTopicUpsertInputDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		upsertInput, err := input.Adapt(organizationId, params.ObjectType, params.ObjectID)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewEntityAnnotationUsecase()
		annotation, err := usecase.UpsertRiskTopicAnnotation(ctx, upsertInput)
		if presentError(ctx, c, err) {
			return
		}

		result, err := dto.AdaptRiskTopicAnnotationToObjectMetadataDto(annotation)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
