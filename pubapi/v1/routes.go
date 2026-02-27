package v1

import (
	"github.com/checkmarble/marble-backend/api/middleware"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/pubapi/types"
	"github.com/checkmarble/marble-backend/pubapi/v1/v1beta"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func Routes(conf pubapi.Config, version string, unauthed *gin.RouterGroup, authMiddleware gin.HandlerFunc, uc usecases.Usecases) {
	unauthed.GET("/-/version", handleVersion(version))

	authed := unauthed.Group("/", authMiddleware, middleware.PrometheusMiddleware)

	{
		root := authed.Group("/", pubapi.TimeoutMiddleware(conf.DefaultTimeout))
		decision := authed.Group("/", pubapi.TimeoutMiddleware(conf.DecisionTimeout))

		root.POST("/ingest/:objectType", HandleIngestObject(uc, false))
		root.PATCH("/ingest/:objectType", HandleIngestObject(uc, false))
		root.POST("/ingest/:objectType/batch", HandleIngestObject(uc, true))
		root.PATCH("/ingest/:objectType/batch", HandleIngestObject(uc, true))

		root.GET("/decisions", HandleListDecisions(uc))
		root.GET("/decisions/:decisionId", HandleGetDecision(uc))
		root.POST("/decisions/:decisionId/snooze", HandleSnoozeRule(uc))
		root.GET("/decisions/:decisionId/screenings", HandleListScreenings(uc))

		decision.POST("/decisions", HandleCreateDecision(uc))
		decision.POST("/decisions/all", HandleCreateAllDecisions(uc))

		root.GET("/batch-executions", HandleListBatchExecutions(uc))

		root.POST("/screening/:screeningId/refine", HandleRefineScreening(uc, true))
		root.POST("/screening/:screeningId/search", HandleRefineScreening(uc, false))
		root.POST("/screening/search", HandleScreeningFreeformSearch(uc))

		root.GET("/screening/entities/:entityId", HandleGetScreeningEntity(uc))
		root.POST("/screening/matches/:matchId", HandleUpdateScreeningMatchStatus(uc))

		root.POST("/screening/whitelists/search", HandleSearchWhitelist(uc))
		root.POST("/screening/whitelists", HandleAddWhitelist(uc))
		root.DELETE("/screening/whitelists", HandleDeleteWhitelist(uc))
	}
}

func BetaRoutes(conf pubapi.Config, unauthed *gin.RouterGroup, authMiddleware gin.HandlerFunc, uc usecases.Usecases) {
	authed := unauthed.Group("/", authMiddleware, middleware.PrometheusMiddleware)

	{
		root := authed.Group("/", pubapi.TimeoutMiddleware(conf.DefaultTimeout))

		root.POST("/ingest/:objectType", v1beta.HandleIngestObject(uc, false))
		root.PATCH("/ingest/:objectType", v1beta.HandleIngestObject(uc, false))
		root.POST("/ingest/:objectType/batch", v1beta.HandleIngestObject(uc, true))
		root.PATCH("/ingest/:objectType/batch", v1beta.HandleIngestObject(uc, true))

		root.POST("/decisions/:decisionId/case", HandleAddDecisionToCase(uc))

		root.GET("/cases", HandleListCases(uc))
		root.GET("/cases/:caseId", HandleGetCase(uc))
		root.POST("/cases", HandleCreateCase(uc))
		root.PATCH("/cases/:caseId", HandleUpdateCase(uc))
		root.POST("/cases/:caseId/close", HandleSetCaseStatus(uc, models.CaseClosed))
		root.POST("/cases/:caseId/reopen", HandleSetCaseStatus(uc, models.CaseInvestigating))
		root.POST("/cases/:caseId/escalate", HandleEscalateCase(uc))
		root.GET("/cases/:caseId/comments", HandleListCaseComments(uc))
		root.POST("/cases/:caseId/comments", HandleCreateComment(uc))
		root.GET("/cases/:caseId/files", HandleListCaseFiles(uc))
		root.POST("/cases/:caseId/files", HandleCreateCaseFile(uc))
		root.GET("/cases/:caseId/files/:fileId/download", HandleDownloadCaseFile(uc))

		root.POST("/continuous-screenings/objects",
			HandleCreateContinuousScreeningObject(uc))
		root.DELETE("/continuous-screenings/objects",
			HandleDeleteContinuousScreeningObject(uc))

		root.GET("/client-data/object-type/:objectType/object-id/:objectId/annotations",
			v1beta.HandleGetClientDataAnnotations(uc))
		root.POST("/client-data/object-type/:objectType/object-id/:objectId/annotations",
			v1beta.HandleAttachClientDataAnnotation(uc))
		root.POST("/client-data/object-type/:objectType/object-id/:objectId/annotations/files",
			v1beta.HandleCreateEntityFileAnnotation(uc))
		root.GET("/client-data/annotations/:id/files/:partId/download",
			v1beta.HandleGetEntityFileAnnotation(uc))
	}
}

func handleVersion(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		types.NewResponse(gin.H{"version": version}).Serve(c)
	}
}
