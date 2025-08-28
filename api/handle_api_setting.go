package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
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

		c.JSON(http.StatusOK)
	}
}
