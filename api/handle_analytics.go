package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleListAnalytics(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewAnalyticsUseCase()
	analytics, err := usecase.ListAnalytics(c.Request.Context())
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analytics": utils.Map(analytics, dto.AdaptAnalyticsDto),
	})
}
