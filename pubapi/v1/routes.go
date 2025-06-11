package v1

import (
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func Routes(conf pubapi.Config, unauthed *gin.RouterGroup, authMiddleware gin.HandlerFunc, uc usecases.Usecases) {
	unauthed.GET("/-/version", handleVersion)

	authed := unauthed.Group("/", authMiddleware)

	{
		root := authed.Group("/", pubapi.TimeoutMiddleware(conf.DefaultTimeout))
		decision := authed.Group("/", pubapi.TimeoutMiddleware(conf.DecisionTimeout))

		root.GET("/decisions", HandleListDecisions(uc))
		root.GET("/decisions/:decisionId", HandleGetDecision(uc))
		root.POST("/decisions/:decisionId/snooze", HandleSnoozeRule(uc))
		root.GET("/decisions/:decisionId/screenings", HandleListSanctionChecks(uc))

		decision.POST("/decisions", HandleCreateDecision(uc))
		decision.POST("/decisions/all", HandleCreateAllDecisions(uc))

		root.GET("/batch-executions", HandleListBatchExecutions(uc))

		root.POST("/screening/:screeningId/refine", HandleRefineSanctionCheck(uc, true))
		root.POST("/screening/:screeningId/search", HandleRefineSanctionCheck(uc, false))
		root.POST("/screening/search", HandleSanctionFreeformSearch(uc))

		root.GET("/screening/entities/:entityId", HandleGetSanctionCheckEntity(uc))
		root.POST("/screening/matches/:matchId", HandleUpdateSanctionCheckMatchStatus(uc))

		root.POST("/screening/whitelists/search", HandleSearchWhitelist(uc))
		root.POST("/screening/whitelists", HandleAddWhitelist(uc))
		root.DELETE("/screening/whitelists", HandleDeleteWhitelist(uc))
	}
}

func handleVersion(c *gin.Context) {
	pubapi.NewResponse(gin.H{"version": "v1beta"}).Serve(c)
}
