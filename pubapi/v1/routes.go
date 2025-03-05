package v1

import (
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.RouterGroup, authMiddleware gin.HandlerFunc, uc usecases.Usecases) {
	r.GET("/-/version", handleVersion)

	{
		r := r.Group("/", authMiddleware)

		r.GET("/decisions/:decisionId/sanction-checks", HandleListSanctionChecks(uc))
		r.POST("/decisions/:decisionId/sanction-checks/refine", HandleRefineSanctionCheck(uc))
		r.POST("/sanction-checks/matches/:matchId",
			HandleUpdateSanctionCheckMatchStatus(uc))
	}
}

func handleVersion(c *gin.Context) {
	pubapi.NewResponse(gin.H{"version": "v1a"}).Serve(c)
}
