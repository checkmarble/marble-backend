package api

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/pubapi"
	pubapiv1 "github.com/checkmarble/marble-backend/pubapi/v1"
	uauth "github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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

func addDefaultRoutes(r *gin.Engine, conf Configuration, uc usecases.Usecases) {
	tom := timeoutMiddleware(conf.DefaultTimeout)

	r.GET("/liveness", tom, HandleLivenessProbe(uc))
	r.GET("/health", tom, handleHealth(uc))
	r.GET("/version", tom, handleVersion(uc))
}

func addRoutes(r *gin.Engine, conf Configuration, uc usecases.Usecases, auth utils.Authentication, tokenHandler TokenHandler, logger *slog.Logger) {
	tom := timeoutMiddleware(conf.DefaultTimeout)
	parsedAppUrl, err := url.Parse(conf.MarbleAppUrl)
	if err != nil || parsedAppUrl.Scheme == "" || parsedAppUrl.Host == "" {
		logger.Error("Failed to parse the Marble app URL environment variable. The decision page url passed in the decisions API response will be empty.", "url", conf.MarbleAppUrl)
	}

	allowedNetworksGuard := uc.NewAllowedNetworksUsecase()

	r.POST("/token", tom, allowedNetworksGuard.Guard(usecases.AllowedNetworksLogin), tokenHandler.GenerateToken)
	r.GET("/config", tom, handleGetConfig(uc, conf))
	r.GET("/is-sso-available", tom, handleIsSSOEnabled(uc))
	r.GET("/signup-status", tom, handleSignupStatus(uc))
	r.GET("/validate-license/*license_key", tom, handleValidateLicense(uc))

	if conf.TokenProvider == uauth.TokenProviderOidc {
		r.POST("/oidc/token", tom, handleOidcTokenExchange(uc, conf.OidcConfig))
	}

	if conf.EnablePrometheus {
		r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	if os.Getenv("DEBUG_ENABLE_PROFILING") == "1" {
		utils.SetupProfilerEndpoints(r, "marble-backend", conf.AppVersion, conf.GcpConfig.ProjectId)
	}

	if infra.IsMarbleSaasProject() {
		r.POST("/metrics", tom, handleMetricsIngestion(uc))
	}

	// Public API initialization
	{
		cfg := pubapi.Config{
			DefaultTimeout:  conf.DefaultTimeout,
			DecisionTimeout: conf.DecisionTimeout,
		}

		pubapi.InitPublicApi()

		pubapiv1.Routes(cfg, "v1", r.Group("/v1"),
			auth.AuthedBy(utils.PublicApiKey, utils.ApiKeyAsBearerToken), uc)

		// Mount both the v1 and new v1beta routes under /v1beta for backward compatibility
		pubapiv1.Routes(cfg, "v1beta", r.Group("/v1beta"),
			auth.AuthedBy(utils.PublicApiKey, utils.ApiKeyAsBearerToken), uc)
		pubapiv1.BetaRoutes(cfg, r.Group("/v1beta"),
			auth.AuthedBy(utils.PublicApiKey, utils.ApiKeyAsBearerToken), uc)
	}

	router := r.Use(auth.AuthedBy(utils.FederatedBearerToken, utils.PublicApiKey),
		allowedNetworksGuard.Guard(usecases.AllowedNetworksOther))

	router.GET("/credentials", tom, handleGetCredentials())

	router.GET("/decisions",
		timeoutMiddleware(conf.DecisionTimeout),
		handleListDecisions(uc, parsedAppUrl))
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

	router.GET("/client_data/:object_type/:object_id", tom, handleGetIngestedObject(uc))
	router.GET("/client_data/:object_type/:object_id/annotations", tom, handleListEntityAnnotations(uc))
	router.GET("/client_data/annotations/:id", tom, handleGetEntityAnnotation(uc))
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
	router.POST("/scenarios/:scenario_id/ast-ai-description", timeoutMiddleware(conf.BatchTimeout),
		handleAiDescriptionAST(uc),
	)
	router.GET("/scenarios/:scenario_id/rules/latest", tom, listLatestScenarioRules(uc))

	router.GET("/scenario-iterations", tom, handleListScenarioIterations(uc))
	router.POST("/scenario-iterations", tom, handleCreateScenarioIteration(uc))
	router.GET("/scenario-iterations/:iteration_id", tom, handleGetScenarioIteration(uc))
	router.POST("/scenario-iterations/:iteration_id", tom, handleCreateDraftFromIteration(uc))
	router.PATCH("/scenario-iterations/:iteration_id", tom, handleUpdateScenarioIteration(uc))
	// Deprecated
	router.POST("/scenario-iterations/:iteration_id/sanction-check", tom, handleCreateScreeningConfig(uc))
	router.PATCH("/scenario-iterations/:iteration_id/sanction-check/:config_id", tom, handleUpdateScreeningCheckConfig(uc))
	router.DELETE("/scenario-iterations/:iteration_id/sanction-check/:config_id", tom, handleDeleteScreeningConfig(uc))
	// New endpoints
	router.POST("/scenario-iterations/:iteration_id/screening", tom, handleCreateScreeningConfig(uc))
	router.PATCH("/scenario-iterations/:iteration_id/screening/:config_id", tom, handleUpdateScreeningCheckConfig(uc))
	router.DELETE("/scenario-iterations/:iteration_id/screening/:config_id", tom, handleDeleteScreeningConfig(uc))
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
	router.GET("/scenario-iteration-rules/:rule_id/ai-description", timeoutMiddleware(conf.BatchTimeout),
		handleAiDescriptionScenarioIteration(uc),
	)

	// Deprecated
	router.GET("/sanction-checks/freshness", tom, handleScreeningDatasetFreshness(uc))
	router.GET("/sanction-checks/datasets", tom, handleScreeningDatasetCatalog(uc))
	router.GET("/sanction-checks", tom, handleListScreenings(uc))
	router.POST("/sanction-checks/refine", tom, handleRefineScreening(uc))
	router.POST("/sanction-checks/search", tom, handleSearchScreening(uc))
	router.POST("/sanction-checks/:screeningId/files", tom,
		handleUploadScreeningMatchFile(uc))
	router.GET("/sanction-checks/:screeningId/files", tom,
		handleListScreeningMatchFiles(uc))
	router.GET("/sanction-checks/:screeningId/files/:fileId", tom,
		handleDownloadScreeningMatchFile(uc))
	router.PATCH("/sanction-checks/matches/:id", tom, handleUpdateScreeningMatchStatus(uc))
	router.POST("/sanction-checks/matches/:id/enrich", tom, handleEnrichScreeningMatch(uc))

	// New endpoints
	router.GET("/screenings/freshness", tom, handleScreeningDatasetFreshness(uc))
	router.GET("/screenings/datasets", tom, handleScreeningDatasetCatalog(uc))
	router.GET("/screenings", tom, handleListScreenings(uc))
	router.POST("/screenings/refine", tom, handleRefineScreening(uc))
	router.POST("/screenings/search", tom, handleSearchScreening(uc))
	router.POST("/screenings/:screeningId/files", tom,
		handleUploadScreeningMatchFile(uc))
	router.GET("/screenings/:screeningId/files", tom,
		handleListScreeningMatchFiles(uc))
	router.GET("/screenings/:screeningId/files/:fileId", tom,
		handleDownloadScreeningMatchFile(uc))
	router.PATCH("/screenings/matches/:id", tom, handleUpdateScreeningMatchStatus(uc))
	router.POST("/screenings/matches/:id/enrich", tom, handleEnrichScreeningMatch(uc))

	router.GET("/screening-monitoring/configs", tom, handleListScreeningMonitoringConfigs(uc))
	router.POST("/screening-monitoring/configs", tom, handleCreateScreeningMonitoringConfig(uc))
	router.GET("/screening-monitoring/configs/:config_id", tom, handleGetScreeningMonitoringConfig(uc))
	router.PATCH("/screening-monitoring/configs/:config_id", tom,
		handleUpdateScreeningMonitoringConfig(uc),
	)
	router.POST("/screening-monitoring/objects", tom, handleInsertScreeningMonitoringObject(uc))

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
	router.PUT("/organizations/:organization_id/subnets", tom, handleUpdateOrganizationSubnets(uc))

	// TODO: deprecated, still used by the back-office. Modify back-office to use the new endpoint below with organization_id query param
	router.GET("/organizations/:organization_id/feature_access", tom, handleGetOrganizationFeatureAccess(uc))
	router.GET("/feature_access", tom, handleGetOrganizationFeatureAccess(uc))
	router.PATCH("/organizations/:organization_id/feature_access", tom,
		handlePatchOrganizationFeatureAccess(uc))

	router.GET("/partners", tom, handleListPartners(uc))
	router.POST("/partners", tom, handleCreatePartner(uc))
	router.GET("/partners/:partner_id", tom, handleGetPartner(uc))
	router.PATCH("/partners/:partner_id", tom, handleUpdatePartner(uc))

	router.GET("/cases", tom, handleListCases(uc))
	router.POST("/cases", tom, handlePostCase(uc))
	router.POST("/cases/mass-update", tom, handleCaseMassUpdate(uc))
	router.GET("/cases/:case_id", tom, handleGetCase(uc))
	router.GET("/cases/:case_id/next", tom, handleGetNextCase(uc))
	router.POST("/cases/:case_id/snooze", tom, handleSnoozeCase(uc))
	router.DELETE("/cases/:case_id/snooze", tom, handleUnsnoozeCase(uc))
	router.PATCH("/cases/:case_id", tom, handlePatchCase(uc))
	router.GET("/cases/:case_id/decisions", tom, handleListCaseDecisions(uc, parsedAppUrl))
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
	router.GET("/cases/:case_id/sar", tom, handleListSuspiciousActivityReports(uc))
	router.POST("/cases/:case_id/sar", tom, handleCreateSuspiciousActivityReport(uc))
	router.PATCH("/cases/:case_id/sar/:reportId", tom, handleUpdateSuspiciousActivityReport(uc))
	router.GET("/cases/:case_id/sar/:reportId/download", tom,
		handleDownloadFileToSuspiciousActivityReport(uc))
	router.DELETE("/cases/:case_id/sar/:reportId", tom,
		handleDeleteSuspiciousActivityReport(uc))
	router.POST("/cases/:case_id/escalate", tom, handleEscalateCase(uc))

	router.GET("/cases/:case_id/data_for_investigation", timeoutMiddleware(conf.BatchTimeout), handleGetCaseDataForCopilot(uc))
	router.GET("/cases/:case_id/review", tom, handleGetCaseReview(uc))
	router.PUT("/cases/:case_id/review/:review_id/feedback", tom, handlePutCaseReviewFeedback(uc))
	router.POST("/cases/:case_id/review/enqueue", tom, handleEnqueueCaseReview(uc))
	router.POST("/cases/:case_id/enrich_kyc", timeoutMiddleware(conf.BatchTimeout), handleEnrichCasePivotObjects(uc))

	router.GET("/inboxes/:inbox_id", tom, handleGetInboxById(uc))
	router.GET("/inboxes/:inbox_id/metadata", tom, handleGetInboxMetadataById(uc))
	router.PATCH("/inboxes/:inbox_id", tom, handlePatchInbox(uc))
	router.DELETE("/inboxes/:inbox_id", tom, handleDeleteInbox(uc))
	router.GET("/inboxes", tom, handleListInboxes(uc))
	router.GET("/inboxes/metadata", tom, handleListInboxesMetadata(uc))
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
	router.GET("/data-model/openapi/:version", tom, handleGetOpenAPI(uc))
	router.POST("/data-model/pivots", tom, handleCreateDataModelPivot(uc))
	router.GET("/data-model/pivots", tom, handleListDataModelPivots(uc))
	router.POST("/data-model/tables/:tableID/navigation_options", tom, handleCreateNavigationOption(uc))
	router.GET("/data-model/tables/:tableID/options", tom, handleGetDataModelOptions(uc))
	router.POST("/data-model/tables/:tableID/options", tom, handleSetDataModelOptions(uc))
	router.GET("/data-model/tables/:tableID/exported-fields", tom, handleGetFieldExportedFields(uc))
	router.POST("/data-model/tables/:tableID/exported-fields", tom, handleCreateFieldExportedFields(uc))

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

	router.GET("/workflows/:scenarioId", tom, handleListWorkflowsForScenario(uc))
	router.POST("/workflows/:scenarioId/reorder", tom, handleReorderWorkflowRules(uc))
	router.POST("/workflows/rule", tom, handleCreateWorkflowRule(uc))
	router.GET("/workflows/rule/:ruleId", tom, handleGetWorkflowRule(uc))
	router.PUT("/workflows/rule/:ruleId", tom, handleUpdateWorkflowRule(uc))
	router.DELETE("/workflows/rule/:ruleId", tom, handleDeleteWorkflowRule(uc))
	router.POST("/workflows/rule/:ruleId/condition", tom, handleCreateWorkflowCondition(uc))
	router.PUT("/workflows/rule/:ruleId/condition/:id", tom, handleUpdateWorkflowCondition(uc))
	router.DELETE("/workflows/rule/:ruleId/condition/:id", tom, handleDeleteWorkflowCondition(uc))
	router.POST("/workflows/rule/:ruleId/action", tom, handleCreateWorkflowAction(uc))
	router.PUT("/workflows/rule/:ruleId/action/:id", tom, handleUpdateWorkflowAction(uc))
	router.DELETE("/workflows/rule/:ruleId/action/:id", tom, handleDeleteWorkflowAction(uc))

	router.GET("/settings/me/unavailable", tom, handleGetUnavailability(uc))
	router.POST("/settings/me/unavailable", tom, handleSetUnavailability(uc))
	router.DELETE("/settings/me/unavailable", tom, handleDeleteUnavailability(uc))

	router.GET("/settings/ai", tom, HandleGetAiSettingForOrganization(uc))
	router.PUT("/settings/ai", tom, HandlePutAiSettingForOrganization(uc))

	if conf.AnalyticsEnabled {
		if conf.AnalyticsProxyApiUrl == "" {
			addAnalyticsRoutes(router, conf, uc)
		} else {
			addAnalyticsProxyRoutes(router, conf)
		}
	}
}

func runStandaloneAnalyticsRoutes(router gin.IRoutes, conf Configuration, uc usecases.Usecases, auth utils.Authentication) {
	allowedNetworksGuard := uc.NewAllowedNetworksUsecase()

	router = router.Use(auth.AuthedBy(utils.FederatedBearerToken, utils.PublicApiKey),
		allowedNetworksGuard.Guard(usecases.AllowedNetworksOther))

	addAnalyticsRoutes(router, conf, uc)
}

func addAnalyticsRoutes(router gin.IRoutes, conf Configuration, uc usecases.Usecases) {
	tom := timeoutMiddleware(conf.AnalyticsTimeout)

	router.POST("/analytics/query/:query", tom, handleAnalyticsQuery(uc))
	router.POST("/analytics/available-filters", tom, handleAnalyticsAvailableFilters(uc))
}

func addAnalyticsProxyRoutes(router gin.IRoutes, conf Configuration) {
	tom := timeoutMiddleware(conf.AnalyticsTimeout)

	router.Match([]string{http.MethodPost}, "/analytics/*path", tom, handleAnalyticsProxy(conf.AnalyticsProxyApiUrl))
}
