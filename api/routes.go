package api

import (
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/checkmarble/marble-backend/pubapi"
	pubapiv1 "github.com/checkmarble/marble-backend/pubapi/v1"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/checkmarble/marble-backend/usecases"

	limits "github.com/gin-contrib/size"
	"github.com/gin-gonic/gin"
	timeout "github.com/vearne/gin-timeout"
)

const maxCaseFileSize = 30 * 1024 * 1024 // 30MB

func timeoutMiddleware(duration time.Duration) gin.HandlerFunc {
	return timeout.Timeout(
		timeout.WithTimeout(duration),
		timeout.WithErrorHttpCode(http.StatusRequestTimeout),
		timeout.WithDefaultMsg("Request timeout"),
	)
}

func addRoutes(r *gin.Engine, conf Configuration, uc usecases.Usecases, auth utils.Authentication, tokenHandler TokenHandler, logger *slog.Logger) {
	tom := timeoutMiddleware(conf.DefaultTimeout)
	parsedAppUrl, err := url.Parse(conf.MarbleAppUrl)
	if err != nil || parsedAppUrl.Scheme == "" || parsedAppUrl.Host == "" {
		logger.Error("Failed to parse the Marble app URL environment variable. The decision page url passed in the decisions API response will be empty.", "url", conf.MarbleAppUrl)
	}

	r.GET("/liveness", tom, handleLivenessProbe(uc))
	r.GET("/version", tom, handleVersion(uc))
	r.POST("/token", tom, tokenHandler.GenerateToken)
	r.GET("/validate-license/*license_key", tom, handleValidateLicense(uc))
	r.GET("/is-sso-available", tom, handleIsSSOEnabled(uc))
	r.GET("/signup-status", tom, handleSignupStatus(uc))

	// Public API initialization
	{
		pubapi.InitPublicApi()
		pubapiv1.Routes(r.Group("/v1beta"), auth.AuthedBy(utils.PublicApiKey, utils.BearerToken), uc)
	}

	router := r.Use(auth.AuthedBy(utils.FederatedBearerToken, utils.PublicApiKey))

	router.GET("/credentials", tom, handleGetCredentials())

	router.GET("/decisions",
		timeoutMiddleware(conf.DecisionTimeout),
		handleListDecisions(uc, parsedAppUrl))
	router.GET("/decisions/with-ranks", tom,
		handleListDecisionsInternal(uc, parsedAppUrl))
	router.POST("/decisions", timeoutMiddleware(conf.DecisionTimeout),
		handlePostDecision(uc, parsedAppUrl))
	router.POST("/decisions/all",
		timeoutMiddleware(3*conf.DecisionTimeout),
		handlePostAllDecisions(uc, parsedAppUrl))
	router.GET("/decisions/:decision_id", tom, handleGetDecision(uc, parsedAppUrl))
	router.GET("/decisions/:decision_id/active-snoozes", tom, handleSnoozesOfDecision(uc))
	router.POST("/decisions/:decision_id/snooze", tom, handleSnoozeDecision(uc))

	router.POST("/ingestion/:object_type", tom, handleIngestion(uc))
	router.PATCH("/ingestion/:object_type", tom, handleIngestionPartialUpsert(uc))
	router.POST("/ingestion/:object_type/multiple", tom, handleIngestionMultiple(uc))
	router.PATCH("/ingestion/:object_type/multiple", tom,
		handleIngestionMultiplePartialUpsert(uc))
	router.POST("/ingestion/:object_type/batch", timeoutMiddleware(conf.BatchTimeout), handlePostCsvIngestion(uc))
	router.GET("/ingestion/:object_type/upload-logs", tom, handleListUploadLogs(uc))
	// TODO: remove deprecated endpoints
	router.GET("/ingestion/:object_type/:object_id", tom, handleGetIngestedObject(uc)) // deprecated, use "/client_data/..."

	router.GET("/client_data/:object_type/:object_id", tom, handleGetIngestedObject(uc))
	router.GET("/client_data/:object_type/:object_id/annotations", tom, handleListEntityAnnotations(uc))
	router.POST("/client_data/:object_type/annotations", tom,
		handleListEntityAnnotationsForObjects(uc))
	router.POST("/client_data/:object_type/:object_id/annotations", tom, handleCreateEntityAnnotation(uc))
	router.POST("/client_data/:object_type/:object_id/annotations/file", tom, handleCreateEntityFileAnnotation(uc))
	router.POST("/client_data/:object_type/list", tom, handleReadClientDataAsList(uc))

	router.GET("/annotations/file/:annotationId/:partId", tom, handleGetEntityFileAnnotation(uc))
	router.DELETE("/annotations/:annotationId", tom, handleDeleteEntityAnnotation(uc))

	router.GET("/scenarios", tom, listScenarios(uc))
	router.POST("/scenarios", tom, createScenario(uc))
	router.GET("/scenarios/:scenario_id", tom, getScenario(uc))
	router.PATCH("/scenarios/:scenario_id", tom, updateScenario(uc))
	router.POST("/scenarios/:scenario_id/validate-ast", tom, validateScenarioAst(uc))

	router.GET("/scenario-iterations", tom, handleListScenarioIterations(uc))
	router.POST("/scenario-iterations", tom, handleCreateScenarioIteration(uc))
	router.GET("/scenario-iterations/:iteration_id", tom, handleGetScenarioIteration(uc))
	router.POST("/scenario-iterations/:iteration_id", tom, handleCreateDraftFromIteration(uc))
	router.PATCH("/scenario-iterations/:iteration_id", tom, handleUpdateScenarioIteration(uc))
	router.PATCH("/scenario-iterations/:iteration_id/sanction-check", tom, handleConfigureSanctionCheck(uc))
	router.DELETE("/scenario-iterations/:iteration_id/sanction-check", tom, handleDeleteSanctionCheckConfig(uc))
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

	router.GET("/sanction-checks/freshness", tom, handleSanctionCheckDatasetFreshness(uc))
	router.GET("/sanction-checks/datasets", tom, handleSanctionCheckDatasetCatalog(uc))
	router.GET("/sanction-checks", tom, handleListSanctionChecks(uc))
	router.POST("/sanction-checks/refine", tom, handleRefineSanctionCheck(uc))
	router.POST("/sanction-checks/search", tom, handleSearchSanctionCheck(uc))
	router.POST("/sanction-checks/:sanctionCheckId/files", tom,
		handleUploadSanctionCheckMatchFile(uc))
	router.GET("/sanction-checks/:sanctionCheckId/files", tom,
		handleListSanctionCheckMatchFiles(uc))
	router.GET("/sanction-checks/:sanctionCheckId/files/:fileId", tom,
		handleDownloadSanctionCheckMatchFile(uc))
	router.PATCH("/sanction-checks/matches/:id", tom, handleUpdateSanctionCheckMatchStatus(uc))
	router.POST("/sanction-checks/matches/:id/enrich", tom, handleEnrichSanctionCheckMatch(uc))

	router.GET("/scenario-publications", tom, handleListScenarioPublications(uc))
	router.POST("/scenario-publications", tom, handleCreateScenarioPublication(uc))
	router.GET("/scenario-publications/preparation", tom,
		handleGetPublicationPreparationStatus(uc))
	router.POST("/scenario-publications/preparation", tom, handleStartPublicationPreparation(uc))
	router.GET("/scenario-publications/:publication_id", tom, handleGetScenarioPublication(uc))

	router.POST("/scenario-testrun", tom, handleCreateScenarioTestRun(uc))
	router.GET("/scenario-testrun", tom, handleListScenarioTestRun(uc))
	router.GET("/scenario-testruns/:test_run_id/decision_data_by_score",
		timeoutMiddleware(conf.BatchTimeout),
		handleDecisionsDataByOutcomeAndScore(uc))
	router.GET("/scenario-testruns/:test_run_id/data_by_rule_execution",
		timeoutMiddleware(conf.BatchTimeout),
		handleTestRunStatsByRulesExecution(uc))
	router.GET("/scenario-testruns/:test_run_id", tom, handleGetScenarioTestRun(uc))
	router.POST("/scenario-testruns/:test_run_id/cancel", tom, handleCancelScenarioTestRun(uc))

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
	router.GET("/custom-lists/:list_id/values", tom, handleGetCsvCustomListValues(uc))
	router.POST("/custom-lists/:list_id/values", tom, handlePostCustomListValue(uc))
	router.POST("/custom-lists/:list_id/values/batch", tom, handlePostCsvCustomListValues(uc))
	router.DELETE("/custom-lists/:list_id/values/:value_id", tom, handleDeleteCustomListValue(uc))

	router.GET("/editor/:scenario_id/identifiers", tom, handleGetEditorIdentifiers(uc))

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
	router.GET("/organizations/:organization_id/feature_access", tom, handleGetOrganizationFeatureAccess(uc))
	router.PATCH("/organizations/:organization_id/feature_access", tom,
		handlePatchOrganizationFeatureAccess(uc))

	router.GET("/partners", tom, handleListPartners(uc))
	router.POST("/partners", tom, handleCreatePartner(uc))
	router.GET("/partners/:partner_id", tom, handleGetPartner(uc))
	router.PATCH("/partners/:partner_id", tom, handleUpdatePartner(uc))

	router.GET("/cases", tom, handleListCases(uc))
	router.POST("/cases", tom, handlePostCase(uc))
	router.GET("/cases/:case_id", tom, handleGetCase(uc))
	router.POST("/cases/:case_id/snooze", tom, handleSnoozeCase(uc))
	router.DELETE("/cases/:case_id/snooze", tom, handleUnsnoozeCase(uc))
	router.PATCH("/cases/:case_id", tom, handlePatchCase(uc))
	router.POST("/cases/:case_id/decisions", tom, handlePostCaseDecisions(uc))
	router.POST("/cases/:case_id/comments", tom, handlePostCaseComment(uc))
	router.POST("/cases/:case_id/case_tags", tom, handlePostCaseTags(uc))
	router.POST("/cases/:case_id/assignee", tom, handleAssignCase(uc))
	router.DELETE("/cases/:case_id/assignee", tom, handleUnassignCase(uc))
	router.POST("/cases/:case_id/files", tom, limits.RequestSizeLimiter(maxCaseFileSize), handlePostCaseFile(uc))
	router.GET("/cases/files/:case_file_id/download_link", tom, handleDownloadCaseFile(uc))
	router.POST("/cases/review_decision", tom, handleReviewCaseDecisions(uc))
	router.GET("/cases/:case_id/annotations", tom, handleGetAnnotationByCase(uc))
	router.GET("/cases/:case_id/pivot_objects", tom, handleReadCasePivotObjects(uc))
	router.GET("/cases/related/pivot/:pivotValue", tom, handleGetRelatedCases(uc))

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
	router.PATCH("/data-model/tables/:tableID", tom, handleUpdateDataModelTable(uc))
	router.POST("/data-model/links", tom, handleCreateLink(uc))
	router.POST("/data-model/tables/:tableID/fields", tom, handleCreateField(uc))
	router.PATCH("/data-model/fields/:fieldID", tom, handleUpdateDataModelField(uc))
	router.DELETE("/data-model", tom, handleDeleteDataModel(uc))
	router.GET("/data-model/openapi", tom, handleGetOpenAPI(uc))
	router.POST("/data-model/pivots", tom, handleCreateDataModelPivot(uc))
	router.GET("/data-model/pivots", tom, handleListDataModelPivots(uc))
	router.POST("/data-model/tables/:tableID/navigation_options", tom, handleCreateNavigationOption(uc))

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
