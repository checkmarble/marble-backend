package api

import (
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"

	limits "github.com/gin-contrib/size"
	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
)

const maxCaseFileSize = 30 * 1024 * 1024 // 30MB

func timeoutMiddleware(duration time.Duration) gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(duration),
		timeout.WithHandler(func(c *gin.Context) {
			c.Next()
		}),
		timeout.WithResponse(func(c *gin.Context) {
			c.String(http.StatusRequestTimeout, "timeout")
		}),
	)
}

const (
	// Infra timeout is 60sec, so we set it to 55sec in order to gracefully handle the timeout in our code
	BATCH_INGESTION_TIMEOUT = 55 * time.Second

	SEQUENTIAL_DECISION_TIMEOUT = 30 * time.Second
	DEFAULT_TIMEOUT             = 5 * time.Second
)

func addRoutes(r *gin.Engine, auth Authentication, tokenHandler TokenHandler, uc usecases.Usecases, marbleAppHost string) {
	tom := timeoutMiddleware(DEFAULT_TIMEOUT)

	r.GET("/liveness", tom, handleLivenessProbe(uc))
	r.POST("/token", tom, tokenHandler.GenerateToken)
	r.GET("/validate-license/*license_key", tom, handleValidateLicense(uc))

	router := r.Use(auth.Middleware)

	router.GET("/credentials", tom, handleGetCredentials())

	router.GET("/ast-expression/available-functions", tom, handleAvailableFunctions)

	router.GET("/decisions", tom, handleListDecisions(uc, marbleAppHost))
	router.POST("/decisions", timeoutMiddleware(models.DECISION_TIMEOUT), handlePostDecision(uc, marbleAppHost))
	router.POST("/decisions/all",
		timeoutMiddleware(SEQUENTIAL_DECISION_TIMEOUT),
		handlePostAllDecisions(uc, marbleAppHost))
	router.GET("/decisions/:decision_id", tom, handleGetDecision(uc, marbleAppHost))
	router.GET("/decisions/:decision_id/active-snoozes", tom, handleSnoozesOfDecision(uc))
	router.POST("/decisions/:decision_id/snooze", tom, handleSnoozeDecision(uc))

	router.POST("/ingestion/:object_type", tom, handleIngestion(uc))
	router.POST("/ingestion/:object_type/multiple", tom, handleIngestionMultiple(uc))
	router.POST("/ingestion/:object_type/batch", timeoutMiddleware(BATCH_INGESTION_TIMEOUT), handlePostCsvIngestion(uc))
	router.GET("/ingestion/:object_type/upload-logs", tom, handleListUploadLogs(uc))

	router.GET("/scenarios", tom, listScenarios(uc))
	router.POST("/scenarios", tom, createScenario(uc))
	router.GET("/scenarios/:scenario_id", tom, getScenario(uc))
	router.PATCH("/scenarios/:scenario_id", tom, updateScenario(uc))

	router.GET("/scenario-iterations", tom, handleListScenarioIterations(uc))
	router.POST("/scenario-iterations", tom, handleCreateScenarioIteration(uc))
	router.GET("/scenario-iterations/:iteration_id", tom, handleGetScenarioIteration(uc))
	router.POST("/scenario-iterations/:iteration_id", tom, handleCreateDraftFromIteration(uc))
	router.PATCH("/scenario-iterations/:iteration_id", tom, handleUpdateScenarioIteration(uc))
	router.POST("/scenario-iterations/:iteration_id/validate", tom, handleValidateScenarioIteration(uc))
	router.POST("/scenario-iterations/:iteration_id/commit",
		tom,
		handleCommitScenarioIterationVersion(uc))
	router.POST("/scenario-iterations/:iteration_id/schedule-execution", tom, handleCreateScheduledExecution(uc))
	router.GET("/scenario-iterations/:iteration_id/active-snoozes", tom, handleSnoozesOfScenarioIteration(uc))

	router.GET("/scenario-iteration-rules", tom, handleListRules(uc))
	router.POST("/scenario-iteration-rules", tom, handleCreateRule(uc))
	router.GET("/scenario-iteration-rules/:rule_id", tom, handleGetRule(uc))
	router.PATCH("/scenario-iteration-rules/:rule_id", tom, handleUpdateRule(uc))
	router.DELETE("/scenario-iteration-rules/:rule_id", tom, handleDeleteRule(uc))

	router.GET("/scenario-publications", tom, handleListScenarioPublications(uc))
	router.POST("/scenario-publications", tom, handleCreateScenarioPublication(uc))
	router.GET("/scenario-publications/preparation", tom,
		handleGetPublicationPreparationStatus(uc))
	router.POST("/scenario-publications/preparation", tom, handleStartPublicationPreparation(uc))
	router.GET("/scenario-publications/:publication_id", tom, handleGetScenarioPublication(uc))

	router.GET("/scheduled-executions", tom, handleListScheduledExecution(uc))
	router.GET("/scheduled-executions/:execution_id", tom, handleGetScheduledExecution(uc))

	router.GET("/analytics", tom, handleListAnalytics(uc))

	router.GET("/apikeys", tom, handleListApiKeys(uc))
	router.POST("/apikeys", tom, handlePostApiKey(uc))
	router.DELETE("/apikeys/:api_key_id", tom, handleRevokeApiKey(uc))

	router.GET("/custom-lists", tom, handleGetAllCustomLists(uc))
	router.POST("/custom-lists", tom, handlePostCustomList(uc))
	router.GET("/custom-lists/:list_id", tom, handleGetCustomListWithValues(uc))
	router.PATCH("/custom-lists/:list_id", tom, handlePatchCustomList(uc))
	router.DELETE("/custom-lists/:list_id", tom, handleDeleteCustomList(uc))
	router.POST("/custom-lists/:list_id/values", tom, handlePostCustomListValue(uc))
	router.DELETE("/custom-lists/:list_id/values/:value_id", tom, handleDeleteCustomListValue(uc))

	router.GET("/editor/:scenario_id/identifiers", tom, handleGetEditorIdentifiers(uc))
	router.GET("/editor/:scenario_id/operators", tom, handleGetEditorOperators(uc))

	router.GET("/users", tom, handleListUsers(uc))
	router.POST("/users", tom, handlePostUser(uc))
	router.GET("/users/:user_id", tom, handleGetUser(uc))
	router.PATCH("/users/:user_id", tom, handlePatchUser(uc))
	router.DELETE("/users/:user_id", tom, handleDeleteUser(uc))
	router.GET("/organizations/:organization_id/users", tom, handleListUsers(uc)) // TODO: deprecated, use GET /users instead (with query param)

	router.GET("/organizations", tom, handleGetOrganizations(uc))
	router.POST("/organizations", tom, handlePostOrganization(uc))
	router.GET("/organizations/:organization_id", tom, handleGetOrganization(uc))
	router.PATCH("/organizations/:organization_id", tom, handlePatchOrganization(uc))
	router.DELETE("/organizations/:organization_id", tom, handleDeleteOrganization(uc))

	router.GET("/partners", tom, handleListPartners(uc))
	router.POST("/partners", tom, handleCreatePartner(uc))
	router.GET("/partners/:partner_id", tom, handleGetPartner(uc))
	router.PATCH("/partners/:partner_id", tom, handleUpdatePartner(uc))

	router.GET("/cases", tom, handleListCases(uc))
	router.POST("/cases", tom, handlePostCase(uc))
	router.GET("/cases/:case_id", tom, handleGetCase(uc))
	router.PATCH("/cases/:case_id", tom, handlePatchCase(uc))
	router.POST("/cases/:case_id/decisions", tom, handlePostCaseDecisions(uc))
	router.POST("/cases/:case_id/comments", tom, handlePostCaseComment(uc))
	router.POST("/cases/:case_id/case_tags", tom, handlePostCaseTags(uc))
	router.POST("/cases/:case_id/files", tom, limits.RequestSizeLimiter(maxCaseFileSize), handlePostCaseFile(uc))
	router.GET("/cases/files/:case_file_id/download_link", tom, handleDownloadCaseFile(uc))
	router.POST("/cases/review_decision", tom, handleReviewCaseDecisions(uc))

	router.GET("/inboxes/:inbox_id", tom, handleGetInboxById(uc))
	router.PATCH("/inboxes/:inbox_id", tom, handlePatchInbox(uc))
	router.DELETE("/inboxes/:inbox_id", tom, handleDeleteInbox(uc))
	router.GET("/inboxes", tom, handleListInboxes(uc))
	router.POST("/inboxes", tom, handlePostInbox(uc))
	router.GET("/inbox_users", tom, handleListAllInboxUsers(uc))
	router.GET("/inbox_users/:inbox_user_id", tom, handleGetInboxUserById(uc))
	router.PATCH("/inbox_users/:inbox_user_id", tom, handlePatchInboxUser(uc))
	router.DELETE("/inbox_users/:inbox_user_id", tom, handleDeleteInboxUser(uc))
	router.GET("/inboxes/:inbox_id/users", tom, handleListInboxUsers(uc))
	router.POST("/inboxes/:inbox_id/users", tom, handlePostInboxUser(uc))

	router.GET("/tags", tom, handleListTags(uc))
	router.POST("/tags", tom, handlePostTag(uc))
	router.GET("/tags/:tag_id", tom, handleGetTag(uc))
	router.PATCH("/tags/:tag_id", tom, handlePatchTag(uc))
	router.DELETE("/tags/:tag_id", tom, handleDeleteTag(uc))

	router.GET("/data-model", tom, handleGetDataModel(uc))
	router.POST("/data-model/tables", tom, handleCreateTable(uc))
	router.POST("/data-model/links", tom, handleCreateLink(uc))
	router.POST("/data-model/tables/:tableID/fields", tom, handleCreateField(uc))
	router.PATCH("/data-model/fields/:fieldID", tom, handleUpdateDataModelField(uc))
	router.DELETE("/data-model", tom, handleDeleteDataModel(uc))
	router.GET("/data-model/openapi", tom, handleGetOpenAPI(uc))
	router.POST("/data-model/pivots", tom, handleCreateDataModelPivot(uc))
	router.GET("/data-model/pivots", tom, handleListDataModelPivots(uc))

	router.POST("/transfers", tom, handleCreateTransfer(uc))
	router.GET("/transfers", tom, handleQueryTransfers(uc))
	router.PATCH("/transfers/:transfer_id", tom, handleUpdateTransfer(uc))
	router.GET("/transfers/:transfer_id", tom, handleGetTransfer(uc))
	router.POST("/transfers/:transfer_id/score", tom, handleScoreTransfer(uc))

	router.POST("/transfer/alerts", tom, handleCreateTransferAlert(uc))
	router.GET("/transfer/sent/alerts/:alert_id", tom, handleGetTransferAlertSender(uc))
	router.GET("/transfer/received/alerts/:alert_id", tom, handleGetTransferAlertBeneficiary(uc))
	router.GET("/transfer/sent/alerts", tom, handleListTransferAlertsSender(uc))
	router.GET("/transfer/received/alerts", tom, handleListTransferAlertsBeneficiary(uc))
	router.PATCH("/transfer/sent/alerts/:alert_id", tom, handleUpdateTransferAlertSender(uc))
	router.PATCH("/transfer/received/alerts/:alert_id",
		tom,
		handleUpdateTransferAlertBeneficiary(uc))

	router.GET("/licenses", tom, handleListLicenses(uc))
	router.POST("/licenses", tom, handleCreateLicense(uc))
	router.PATCH("/licenses/:license_id", tom, handleUpdateLicense(uc))
	router.GET("/licenses/:license_id", tom, handleGetLicenseById(uc))

	router.GET("/webhooks", tom, handleListWebhooks(uc))
	router.POST("/webhooks", tom, handleRegisterWebhook(uc))
	router.GET("/webhooks/:webhook_id", tom, handleGetWebhook(uc))
	router.PATCH("/webhooks/:webhook_id", tom, handleUpdateWebhook(uc))
	router.DELETE("/webhooks/:webhook_id", tom, handleDeleteWebhook(uc))

	router.GET("/rule-snoozes/:rule_snooze_id", tom, handleGetSnoozesById(uc))
}
