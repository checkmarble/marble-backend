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

func handleGetScreeningMonitoringConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		configId, err := uuid.Parse(c.Param("config_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningMonitoringUsecase()
		screeningMonitoringConfig, err := uc.GetScreeningMonitoringConfig(ctx, configId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptScreeningMonitoringConfigDto(screeningMonitoringConfig))
	}
}

func handleListScreeningMonitoringConfigs(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningMonitoringUsecase()
		screeningMonitoringConfigs, err := uc.GetScreeningMonitoringConfigsByOrgId(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, pure_utils.Map(screeningMonitoringConfigs, dto.AdaptScreeningMonitoringConfigDto))
	}
}

func handleCreateScreeningMonitoringConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreateScreeningMonitoringConfigDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		if err := input.Validate(); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		createScreeningMonitoringConfigInput :=
			dto.AdaptCreateScreeningMonitoringConfigDtoToModel(input)
		createScreeningMonitoringConfigInput.OrgId = organizationId

		uc := usecasesWithCreds(ctx, uc).NewScreeningMonitoringUsecase()
		screeningMonitoringConfig, err := uc.CreateScreeningMonitoringConfig(ctx, createScreeningMonitoringConfigInput)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, dto.AdaptScreeningMonitoringConfigDto(screeningMonitoringConfig))
	}
}

func handleUpdateScreeningMonitoringConfig(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		configId, err := uuid.Parse(c.Param("config_id"))
		if err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		var input dto.UpdateScreeningMonitoringConfigDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		if err := input.Validate(); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningMonitoringUsecase()
		screeningMonitoringConfig, err := uc.UpdateScreeningMonitoringConfig(
			ctx,
			configId,
			dto.AdaptUpdateScreeningMonitoringConfigDtoToModel(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptScreeningMonitoringConfigDto(screeningMonitoringConfig))
	}
}

func handleInsertScreeningMonitoringObject(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var input dto.InsertScreeningMonitoringObjectDto
		if err := c.ShouldBindJSON(&input); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		if err := input.Validate(); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewScreeningMonitoringUsecase()
		err := uc.InsertScreeningMonitoringObject(
			ctx,
			dto.AdaptInsertScreeningMonitoringObjectDtoToModel(input),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusCreated)
	}
}
