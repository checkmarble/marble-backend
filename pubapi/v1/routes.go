package v1

import (
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.RouterGroup, authMiddleware gin.HandlerFunc, uc usecases.Usecases, cfg pubapi.Config) {
	r.GET("/-/version", handleVersion)

	{
		r := r.Group("/", authMiddleware)

		r.GET("/decisions", HandleListDecisions(uc))
		r.GET("/decisions/:decisionId", HandleGetDecision(uc))
		r.POST("/decisions/:decisionId/snooze", HandleSnoozeRule(uc))
		r.GET("/decisions/:decisionId/screenings", HandleListSanctionChecks(uc))

		r.POST("/screening/:screeningId/refine", HandleRefineSanctionCheck(uc, true))
		r.POST("/screening/:screeningId/search", HandleRefineSanctionCheck(uc, false))
		r.POST("/screening/search", HandleSanctionFreeformSearch(uc))

		r.GET("/screening/entities/:entityId", HandleGetSanctionCheckEntity(uc))
		r.POST("/screening/matches/:matchId",
			HandleUpdateSanctionCheckMatchStatus(uc))

		r.POST("/screening/whitelists/search", HandleSearchWhitelist(uc))
		r.POST("/screening/whitelists", HandleAddWhitelist(uc))
		r.DELETE("/screening/whitelists", HandleDeleteWhitelist(uc))
	}
}

func handleVersion(c *gin.Context) {
	pubapi.NewResponse(gin.H{"version": "v1beta"}).Serve(c)
}
