package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleGetFieldExportedFields(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		uc := usecasesWithCreds(ctx, uc).NewAnalyticsSettingsUsecase()

		settings, err := uc.GetAnalyticsSettings(ctx, c.Param("tableID"))

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptAnalyticsSettings(settings))
	}
}

func handleCreateFieldExportedFields(uc usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.AnalyticsSettingDto

		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		uc := usecasesWithCreds(ctx, uc).NewAnalyticsSettingsUsecase()
		settings, err := uc.UpdateAnalyticsSettings(ctx, c.Param("tableID"), payload)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptAnalyticsSettings(settings))
	}
}
