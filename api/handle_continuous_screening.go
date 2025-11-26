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

func handleGetContinuousScreeningConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		stableIdString := c.Param("stable_id")
		stableId, err := uuid.Parse(stableIdString)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfig, err := uc.GetContinuousScreeningConfigByStableId(ctx, stableId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningConfigDto(continuousScreeningConfig))
	}
}

func handleListContinuousScreeningConfigs(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfigs, err := uc.GetContinuousScreeningConfigsByOrgId(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(continuousScreeningConfigs, dto.AdaptContinuousScreeningConfigDto))
	}
}

func handleCreateContinuousScreeningConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationIdString, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}
		organizationId, err := uuid.Parse(organizationIdString)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var input dto.CreateContinuousScreeningConfigDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		if err := input.Validate(); err != nil {
			presentError(ctx, c, err)
			return
		}

		createContinuousScreeningConfigInput :=
			dto.AdaptCreateContinuousScreeningConfigDtoToModel(input)
		createContinuousScreeningConfigInput.OrgId = organizationId

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfig, err := uc.CreateContinuousScreeningConfig(ctx, createContinuousScreeningConfigInput)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptContinuousScreeningConfigDto(continuousScreeningConfig))
	}
}

func handleUpdateContinuousScreeningConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		stableIdString := c.Param("stable_id")
		stableId, err := uuid.Parse(stableIdString)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var input dto.UpdateContinuousScreeningConfigDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		if err := input.Validate(); err != nil {
			presentError(ctx, c, err)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfig, err := uc.UpdateContinuousScreeningConfig(
			ctx,
			stableId,
			dto.AdaptUpdateContinuousScreeningConfigDtoToModel(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningConfigDto(continuousScreeningConfig))
	}
}

func handleCreateContinuousScreeningObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var input dto.CreateContinuousScreeningObjectDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		if err := input.Validate(); err != nil {
			presentError(ctx, c, err)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		screeningResponse, err := uc.InsertContinuousScreeningObject(
			ctx,
			dto.AdaptInsertContinuousScreeningObjectDto(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptScreeningDto(
			screeningResponse,
		))
	}
}

var continuousScreeningPaginationDefaults = models.PaginationDefaults{
	Limit:  25,
	SortBy: models.ContinuousScreeningSortingCreatedAt,
	Order:  models.SortingOrderDesc,
}

func handleListContinuousScreeningsForOrg(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		orgId, err := uuid.Parse(organizationId)
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var paginationAndSortingDto dto.PaginationAndSorting
		if err := c.ShouldBind(&paginationAndSortingDto); err != nil {
			c.JSON(http.StatusBadRequest, dto.APIErrorResponse{
				Message: err.Error(),
			})
			return
		}
		paginationAndSorting := models.WithPaginationDefaults(
			dto.AdaptPaginationAndSorting(paginationAndSortingDto),
			continuousScreeningPaginationDefaults,
		)

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		screenings, err := uc.ListContinuousScreeningsForOrg(ctx, orgId, paginationAndSorting)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(
			http.StatusOK,
			pure_utils.Map(
				screenings,
				dto.AdaptContinuousScreeningDto,
			),
		)
	}
}

func handleUpdateContinuousScreeningMatchStatus(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		matchId := c.Param("id")

		var payload dto.ScreeningMatchUpdateDto
		if presentError(ctx, c, c.ShouldBindJSON(&payload)) {
			return
		}

		creds, ok := utils.CredentialsFromCtx(ctx)
		if !ok {
			presentError(ctx, c, models.ErrUnknownUser)
			return
		}
		update, err := dto.AdaptScreeningMatchUpdateInputDto(matchId, creds.ActorIdentity.UserId, payload)
		if presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		match, err := uc.UpdateContinuousScreeningMatchStatus(ctx, update)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningMatchDto(match))
	}
}

func handleDeleteContinuousScreeningObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var input dto.DeleteContinuousScreeningObjectDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		err := uc.DeleteContinuousScreeningObject(
			ctx,
			dto.AdaptDeleteContinuousScreeningObjectDto(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}
