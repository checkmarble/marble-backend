package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleListAnalytics(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewAnalyticsUseCase()
		analytics, err := usecase.ListAnalytics(c.Request.Context(), organizationId)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"analytics": pure_utils.Map(analytics, dto.AdaptAnalyticsDto),
		})
	}
}
