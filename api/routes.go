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

// Infra timeout is 60sec, so we set it to 55sec in order to gracefully handle the timeout in our code
const batchIngestionTimeout = 55 * time.Second

func addRoutes(r *gin.Engine, auth Authentication, tokenHandler TokenHandler, uc usecases.Usecases, marbleAppHost string) {
	r.GET("/liveness", handleLivenessProbe)
	r.POST("/token", tokenHandler.GenerateToken)
	r.GET("/validate-license/*license_key", handleValidateLicense(uc))

	router := r.Use(auth.Middleware)

	router.GET("/credentials", handleGetCredentials())

	router.GET("/ast-expression/available-functions", handleAvailableFunctions)

	router.GET("/decisions", handleListDecisions(uc, marbleAppHost))
	router.POST("/decisions", timeoutMiddleware(models.DECISION_TIMEOUT), handlePostDecision(uc, marbleAppHost))
	router.POST("/decisions/all", timeoutMiddleware(models.SEQUENTIAL_DECISION_TIMEOUT),
		handlePostAllDecisions(uc, marbleAppHost))
	router.GET("/decisions/:decision_id", handleGetDecision(uc, marbleAppHost))
	router.GET("/decisions/:decision_id/active-snoozes", handleSnoozesOfDecision(uc))
	router.POST("/decisions/:decision_id/snooze", handleSnoozeDecision(uc))

	router.POST("/ingestion/:object_type", handleIngestion(uc))
	router.POST("/ingestion/:object_type/batch", timeoutMiddleware(batchIngestionTimeout), handlePostCsvIngestion(uc))
	router.GET("/ingestion/:object_type/upload-logs", handleListUploadLogs(uc))

	router.GET("/scenarios", listScenarios(uc))
	router.POST("/scenarios", createScenario(uc))
	router.GET("/scenarios/:scenario_id", getScenario(uc))
	router.PATCH("/scenarios/:scenario_id", updateScenario(uc))

	router.GET("/scenario-iterations", handleListScenarioIterations(uc))
	router.POST("/scenario-iterations", handleCreateScenarioIteration(uc))
	router.GET("/scenario-iterations/:iteration_id", handleGetScenarioIteration(uc))
	router.POST("/scenario-iterations/:iteration_id", handleCreateDraftFromIteration(uc))
	router.PATCH("/scenario-iterations/:iteration_id", handleUpdateScenarioIteration(uc))
	router.POST("/scenario-iterations/:iteration_id/validate", handleValidateScenarioIteration(uc))
	router.POST("/scenario-iterations/:iteration_id/commit",
		handleCommitScenarioIterationVersion(uc))
	router.POST("/scenario-iterations/:iteration_id/schedule-execution", handleCreateScheduledExecution(uc))
	router.GET("/scenario-iterations/:iteration_id/active-snoozes", handleSnoozesOfScenarioIteration(uc))

	router.GET("/scenario-iteration-rules", handleListRules(uc))
	router.POST("/scenario-iteration-rules", handleCreateRule(uc))
	router.GET("/scenario-iteration-rules/:rule_id", handleGetRule(uc))
	router.PATCH("/scenario-iteration-rules/:rule_id", handleUpdateRule(uc))
	router.DELETE("/scenario-iteration-rules/:rule_id", handleDeleteRule(uc))

	router.GET("/scenario-publications", handleListScenarioPublications(uc))
	router.POST("/scenario-publications", handleCreateScenarioPublication(uc))
	router.GET("/scenario-publications/preparation", handleGetPublicationPreparationStatus(uc))
	router.POST("/scenario-publications/preparation", handleStartPublicationPreparation(uc))
	router.GET("/scenario-publications/:publication_id", handleGetScenarioPublication(uc))

	router.GET("/scheduled-executions", handleListScheduledExecution(uc))
	router.GET("/scheduled-executions/:execution_id", handleGetScheduledExecution(uc))

	// TODO deprecated
	router.GET("/scheduled-executions/:execution_id/decisions.zip",
		handleGetScheduledExecutionDecisions(uc))

	router.GET("/analytics", handleListAnalytics(uc))

	router.GET("/apikeys", handleListApiKeys(uc))
	router.POST("/apikeys", handlePostApiKey(uc))
	router.DELETE("/apikeys/:api_key_id", handleRevokeApiKey(uc))

	router.GET("/custom-lists", handleGetAllCustomLists(uc))
	router.POST("/custom-lists", handlePostCustomList(uc))
	router.GET("/custom-lists/:list_id", handleGetCustomListWithValues(uc))
	router.PATCH("/custom-lists/:list_id", handlePatchCustomList(uc))
	router.DELETE("/custom-lists/:list_id", handleDeleteCustomList(uc))
	router.POST("/custom-lists/:list_id/values", handlePostCustomListValue(uc))
	router.DELETE("/custom-lists/:list_id/values/:value_id", handleDeleteCustomListValue(uc))

	router.GET("/editor/:scenario_id/identifiers", handleGetEditorIdentifiers(uc))
	router.GET("/editor/:scenario_id/operators", handleGetEditorOperators(uc))

	router.GET("/users", handleGetAllUsers(uc))
	router.POST("/users", handlePostUser(uc))
	router.GET("/users/:user_id", handleGetUser(uc))
	router.PATCH("/users/:user_id", handlePatchUser(uc))
	router.DELETE("/users/:user_id", handleDeleteUser(uc))

	router.GET("/organizations", handleGetOrganizations(uc))
	router.POST("/organizations", handlePostOrganization(uc))
	router.GET("/organizations/:organization_id", handleGetOrganization(uc))
	router.PATCH("/organizations/:organization_id", handlePatchOrganization(uc))
	router.DELETE("/organizations/:organization_id", handleDeleteOrganization(uc))
	router.GET("/organizations/:organization_id/users", handleGetOrganizationUsers(uc))

	router.GET("/partners", handleListPartners(uc))
	router.POST("/partners", handleCreatePartner(uc))
	router.GET("/partners/:partner_id", handleGetPartner(uc))
	router.PATCH("/partners/:partner_id", handleUpdatePartner(uc))

	router.GET("/cases", handleListCases(uc))
	router.POST("/cases", handlePostCase(uc))
	router.GET("/cases/:case_id", handleGetCase(uc))
	router.PATCH("/cases/:case_id", handlePatchCase(uc))
	router.POST("/cases/:case_id/decisions", handlePostCaseDecisions(uc))
	router.POST("/cases/:case_id/comments", handlePostCaseComment(uc))
	router.POST("/cases/:case_id/case_tags", handlePostCaseTags(uc))
	router.POST("/cases/:case_id/files", limits.RequestSizeLimiter(maxCaseFileSize), handlePostCaseFile(uc))
	router.GET("/cases/files/:case_file_id/download_link", handleDownloadCaseFile(uc))
	router.POST("/cases/review_decision", handleReviewCaseDecisions(uc))

	router.GET("/inboxes/:inbox_id", handleGetInboxById(uc))
	router.PATCH("/inboxes/:inbox_id", handlePatchInbox(uc))
	router.DELETE("/inboxes/:inbox_id", handleDeleteInbox(uc))
	router.GET("/inboxes", handleListInboxes(uc))
	router.POST("/inboxes", handlePostInbox(uc))
	router.GET("/inbox_users", handleListAllInboxUsers(uc))
	router.GET("/inbox_users/:inbox_user_id", handleGetInboxUserById(uc))
	router.PATCH("/inbox_users/:inbox_user_id", handlePatchInboxUser(uc))
	router.DELETE("/inbox_users/:inbox_user_id", handleDeleteInboxUser(uc))
	router.GET("/inboxes/:inbox_id/users", handleListInboxUsers(uc))
	router.POST("/inboxes/:inbox_id/users", handlePostInboxUser(uc))

	router.GET("/tags", handleListTags(uc))
	router.POST("/tags", handlePostTag(uc))
	router.GET("/tags/:tag_id", handleGetTag(uc))
	router.PATCH("/tags/:tag_id", handlePatchTag(uc))
	router.DELETE("/tags/:tag_id", handleDeleteTag(uc))

	router.GET("/data-model", handleGetDataModel(uc))
	router.POST("/data-model/tables", handleCreateTable(uc))
	router.POST("/data-model/links", handleCreateLink(uc))
	router.POST("/data-model/tables/:tableID/fields", handleCreateField(uc))
	router.PATCH("/data-model/fields/:fieldID", handleUpdateDataModelField(uc))
	router.DELETE("/data-model", handleDeleteDataModel(uc))
	router.GET("/data-model/openapi", handleGetOpenAPI(uc))
	router.POST("/data-model/pivots", handleCreateDataModelPivot(uc))
	router.GET("/data-model/pivots", handleListDataModelPivots(uc))

	router.POST("/transfers", handleCreateTransfer(uc))
	router.GET("/transfers", handleQueryTransfers(uc))
	router.PATCH("/transfers/:transfer_id", handleUpdateTransfer(uc))
	router.GET("/transfers/:transfer_id", handleGetTransfer(uc))
	router.POST("/transfers/:transfer_id/score", handleScoreTransfer(uc))

	router.POST("/transfer/alerts", handleCreateTransferAlert(uc))
	router.GET("/transfer/sent/alerts/:alert_id", handleGetTransferAlertSender(uc))
	router.GET("/transfer/received/alerts/:alert_id", handleGetTransferAlertBeneficiary(uc))
	router.GET("/transfer/sent/alerts", handleListTransferAlertsSender(uc))
	router.GET("/transfer/received/alerts", handleListTransferAlertsBeneficiary(uc))
	router.PATCH("/transfer/sent/alerts/:alert_id", handleUpdateTransferAlertSender(uc))
	router.PATCH("/transfer/received/alerts/:alert_id",
		handleUpdateTransferAlertBeneficiary(uc))

	router.GET("/licenses", handleListLicenses(uc))
	router.POST("/licenses", handleCreateLicense(uc))
	router.PATCH("/licenses/:license_id", handleUpdateLicense(uc))
	router.GET("/licenses/:license_id", handleGetLicenseById(uc))

	router.GET("/webhooks", handleListWebhooks(uc))
	router.POST("/webhooks", handleRegisterWebhook(uc))
	router.GET("/webhooks/:webhook_id", handleGetWebhook(uc))
	router.PATCH("/webhooks/:webhook_id", handleUpdateWebhook(uc))
	router.DELETE("/webhooks/:webhook_id", handleDeleteWebhook(uc))

	router.GET("/rule-snoozes/:rule_snooze_id", handleGetSnoozesById(uc))
}
