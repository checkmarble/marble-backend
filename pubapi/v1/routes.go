package v1

import (
	"net/http"

	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.RouterGroup, authF gin.HandlerFunc, uc usecases.Usecases) {
	r.GET("/-/version", version(uc))

	{
		r := r.Group("/", authF)

		r.GET("/decisions/:decisionId/sanction-checks", HandleListSanctionChecks(uc))
		r.POST("/sanction-checks/matches/:matchId",
			HandleUpdateSanctionCheckMatchStatus(uc))
	}
}

func version(_ usecases.Usecases) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": "v1"})
	}
}
