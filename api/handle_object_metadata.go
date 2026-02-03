package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var objectMetadataPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.SortingFieldCreatedAt,
	Order:  models.SortingOrderDesc,
}

func handleListObjectMetadata(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var filterDto dto.ObjectMetadataFilterDto
		if err := c.ShouldBindQuery(&filterDto); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var paginationDto dto.PaginationAndSorting
		if err := c.ShouldBindQuery(&paginationDto); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		paginationAndSorting := models.WithPaginationDefaults(
			dto.AdaptPaginationAndSorting(paginationDto),
			objectMetadataPaginationDefaults,
		)

		filter, err := filterDto.Adapt(organizationId)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectMetadataUsecase()
		objectMetadataList, err := usecase.ListObjectMetadata(ctx, filter, paginationAndSorting)
		if presentError(ctx, c, err) {
			return
		}

		dtos := make([]dto.ObjectMetadataDto, len(objectMetadataList))
		for i, m := range objectMetadataList {
			d, err := dto.AdaptObjectMetadataDto(m)
			if presentError(ctx, c, err) {
				return
			}
			dtos[i] = d
		}

		c.JSON(http.StatusOK, dtos)
	}
}

func handleGetObjectRiskTopics(uc usecases.Usecases) func(c *gin.Context) {
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

		usecase := usecasesWithCreds(ctx, uc).NewObjectMetadataUsecase()
		objectRiskTopic, err := usecase.GetObjectRiskTopicByObjectId(ctx, organizationId, objectType, objectId)
		if presentError(ctx, c, err) {
			return
		}

		result, err := dto.AdaptObjectMetadataDto(objectRiskTopic.ObjectMetadata)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, result)
	}
}

func handleUpsertObjectRiskTopics(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, errors.Wrap(models.ForbiddenError, "credentials not found"))
			return
		}

		userId, err := uuid.Parse(string(creds.ActorIdentity.UserId))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid user id"))
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

		var input dto.ObjectRiskTopicUpsertInputDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		upsertInput, err := input.Adapt(organizationId, userId, objectType, objectId)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectMetadataUsecase()
		objectRiskTopic, err := usecase.UpsertObjectRiskTopic(ctx, upsertInput)
		if presentError(ctx, c, err) {
			return
		}

		result, err := dto.AdaptObjectMetadataDto(objectRiskTopic.ObjectMetadata)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, result)
	}
}
