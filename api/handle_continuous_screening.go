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
		configId, err := uuid.Parse(c.Param("config_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewContinuousScreeningUsecase()
		continuousScreeningConfig, err := uc.GetContinuousScreeningConfig(ctx, configId)
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
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
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

		configId, err := uuid.Parse(c.Param("config_id"))
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
			configId,
			dto.AdaptUpdateContinuousScreeningConfigDtoToModel(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptContinuousScreeningConfigDto(continuousScreeningConfig))
	}
}

func handleInsertContinuousScreeningObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var input dto.InsertContinuousScreeningObjectDto
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

		// TODO: Need to define filters

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
