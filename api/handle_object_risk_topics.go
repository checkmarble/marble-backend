package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var objectRiskTopicPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.SortingFieldCreatedAt,
	Order:  models.SortingOrderDesc,
}

func handleListObjectRiskTopics(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var filterDto dto.ObjectRiskTopicFilterDto
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
			objectRiskTopicPaginationDefaults,
		)

		filter, err := filterDto.Adapt(organizationId)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectRiskTopicUsecase()
		objectRiskTopics, err := usecase.ListObjectRiskTopics(ctx, filter, paginationAndSorting)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(objectRiskTopics, dto.AdaptObjectRiskTopicDto))
	}
}

func handleGetObjectRiskTopic(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		objectRiskTopicsId, err := uuid.Parse(c.Param("object_risk_topics_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid object_risk_topics_id"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectRiskTopicUsecase()
		objectRiskTopic, err := usecase.GetObjectRiskTopicById(ctx, objectRiskTopicsId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptObjectRiskTopicDto(objectRiskTopic))
	}
}

func handleUpsertObjectRiskTopic(uc usecases.Usecases) func(c *gin.Context) {
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

		var input dto.ObjectRiskTopicUpsertInputDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		upsertInput, err := input.Adapt(organizationId, userId)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectRiskTopicUsecase()
		err = usecase.UpsertObjectRiskTopic(ctx, upsertInput)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleListObjectRiskTopicEvents(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		objectRiskTopicsId, err := uuid.Parse(c.Param("object_risk_topics_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, "invalid object_risk_topics_id"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewObjectRiskTopicUsecase()
		events, err := usecase.ListObjectRiskTopicEvents(ctx, objectRiskTopicsId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(events, dto.AdaptObjectRiskTopicEventDto))
	}
}
