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

func HandleGetAiSetting(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}
		usecase := usecasesWithCreds(ctx, uc).NewAiSettingUsecase()

		aiSetting, err := usecase.GetAiSetting(ctx, organizationId)
		if presentError(ctx, c, err) {
			return
		}
		if aiSetting == nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, dto.AdaptAiSettingDto(*aiSetting))
	}
}

func HandleUpsertAiSetting(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var payload dto.UpsertAiSettingDto
		if err := c.ShouldBindJSON(&payload); presentError(ctx, c, err) {
			return
		}

		if err := payload.Validate(); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewAiSettingUsecase()
		aiSetting, err := usecase.UpsertAiSetting(ctx, organizationId, dto.AdaptUpsertAiSetting(payload))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptAiSettingDto(aiSetting))
	}
}
