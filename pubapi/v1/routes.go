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

		r.POST("/decisions/:decisionId/snooze", HandleSnoozeRule(uc))
		r.GET("/decisions/:decisionId/sanction-checks", HandleListSanctionChecks(uc))

		r.POST("/sanction-checks/:sanctionCheckId/refine", HandleRefineSanctionCheck(uc, true))
		r.POST("/sanction-checks/:sanctionCheckId/search", HandleRefineSanctionCheck(uc, false))
		r.POST("/sanction-checks/search", HandleSanctionFreeformSearch(uc))

		r.GET("/sanction-checks/entities/:entityId", HandleGetSanctionCheckEntity(uc))
		r.POST("/sanction-checks/matches/:matchId",
			HandleUpdateSanctionCheckMatchStatus(uc))

		r.POST("/sanction-checks/whitelists/search", HandleSearchWhitelist(uc))
		r.POST("/sanction-checks/whitelists", HandleAddWhitelist(uc))
		r.DELETE("/sanction-checks/whitelists", HandleDeleteWhitelist(uc))
	}
}

func handleVersion(c *gin.Context) {
	pubapi.NewResponse(gin.H{"version": "v1beta"}).Serve(c)
}
