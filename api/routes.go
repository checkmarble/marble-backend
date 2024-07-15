package api

import (
	"net/http"
	"time"

	"github.com/checkmarble/marble-backend/models"
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

func (api *API) routes(auth Authentication, tokenHandler TokenHandler) {
	api.router.GET("/liveness", HandleLivenessProbe)
	api.router.POST("/token", tokenHandler.GenerateToken)
	api.router.GET("/validate-license/*license_key", api.handleValidateLicense)

	router := api.router.Use(auth.Middleware)

	router.GET("/credentials", api.handleGetCredentials)

	router.GET("/ast-expression/available-functions", api.handleAvailableFunctions)

	router.GET("/decisions", api.handleListDecisions)
	router.POST("/decisions", timeoutMiddleware(models.DECISION_TIMEOUT), api.handlePostDecision)
	router.POST("/decisions/all", timeoutMiddleware(models.SEQUENTIAL_DECISION_TIMEOUT), api.handlePostAllDecisions)
	router.GET("/decisions/:decision_id", api.handleGetDecision)

	router.POST("/ingestion/:object_type", api.handleIngestion)
	router.POST("/ingestion/:object_type/batch", timeoutMiddleware(batchIngestionTimeout), api.handleCsvIngestion)
	router.GET("/ingestion/:object_type/upload-logs", api.handleListUploadLogs)

	router.GET("/scenarios", api.ListScenarios)
	router.POST("/scenarios", api.CreateScenario)
	router.GET("/scenarios/:scenario_id", api.GetScenario)
	router.PATCH("/scenarios/:scenario_id", api.UpdateScenario)

	router.GET("/scenario-iterations", api.ListScenarioIterations)
	router.POST("/scenario-iterations", api.CreateScenarioIteration)
	router.GET("/scenario-iterations/:iteration_id", api.GetScenarioIteration)
	router.POST("/scenario-iterations/:iteration_id", api.CreateDraftFromIteration)
	router.PATCH("/scenario-iterations/:iteration_id", api.UpdateScenarioIteration)
	router.POST("/scenario-iterations/:iteration_id/validate", api.ValidateScenarioIteration)
	router.POST("/scenario-iterations/:iteration_id/commit", api.CommitScenarioIterationVersion)
	router.POST("/scenario-iterations/:iteration_id/schedule-execution", api.handleCreateScheduledExecution)

	router.GET("/scenario-iteration-rules", api.ListRules)
	router.POST("/scenario-iteration-rules", api.CreateRule)
	router.GET("/scenario-iteration-rules/:rule_id", api.GetRule)
	router.PATCH("/scenario-iteration-rules/:rule_id", api.UpdateRule)
	router.DELETE("/scenario-iteration-rules/:rule_id", api.DeleteRule)

	router.GET("/scenario-publications", api.ListScenarioPublications)
	router.POST("/scenario-publications", api.CreateScenarioPublication)
	router.GET("/scenario-publications/preparation", api.GetPublicationPreparationStatus)
	router.POST("/scenario-publications/preparation", api.StartPublicationPreparation)
	router.GET("/scenario-publications/:publication_id", api.GetScenarioPublication)

	router.GET("/scheduled-executions", api.handleListScheduledExecution)
	router.GET("/scheduled-executions/:execution_id", api.handleGetScheduledExecution)
	router.GET("/scheduled-executions/:execution_id/decisions.zip",
		api.handleGetScheduledExecutionDecisions)

	router.GET("/analytics", api.handleListAnalytics)

	router.GET("/apikeys", api.handleListApiKeys)
	router.POST("/apikeys", api.handlePostApiKey)
	router.DELETE("/apikeys/:api_key_id", api.handleRevokeApiKey)

	router.GET("/custom-lists", api.handleGetAllCustomLists)
	router.POST("/custom-lists", api.handlePostCustomList)
	router.GET("/custom-lists/:list_id", api.handleGetCustomListWithValues)
	router.PATCH("/custom-lists/:list_id", api.handlePatchCustomList)
	router.DELETE("/custom-lists/:list_id", api.handleDeleteCustomList)
	router.POST("/custom-lists/:list_id/values", api.handlePostCustomListValue)
	router.DELETE("/custom-lists/:list_id/values/:value_id", api.handleDeleteCustomListValue)

	router.GET("/editor/:scenario_id/identifiers", api.handleGetEditorIdentifiers)
	router.GET("/editor/:scenario_id/operators", api.handleGetEditorOperators)

	router.GET("/users", api.handleGetAllUsers)
	router.POST("/users", api.handlePostUser)
	router.GET("/users/:user_id", api.handleGetUser)
	router.PATCH("/users/:user_id", api.handlePatchUser)
	router.DELETE("/users/:user_id", api.handleDeleteUser)

	router.GET("/organizations", api.handleGetOrganizations)
	router.POST("/organizations", api.handlePostOrganization)
	router.GET("/organizations/:organization_id", api.handleGetOrganization)
	router.PATCH("/organizations/:organization_id", api.handlePatchOrganization)
	router.DELETE("/organizations/:organization_id", api.handleDeleteOrganization)
	router.GET("/organizations/:organization_id/users", api.handleGetOrganizationUsers)

	router.GET("/partners", api.handleListPartners)
	router.POST("/partners", api.handleCreatePartner)
	router.GET("/partners/:partner_id", api.handleGetPartner)
	router.PATCH("/partners/:partner_id", api.handleUpdatePartner)

	router.GET("/cases", api.handleListCases)
	router.POST("/cases", api.handlePostCase)
	router.GET("/cases/:case_id", api.handleGetCase)
	router.PATCH("/cases/:case_id", api.handlePatchCase)
	router.POST("/cases/:case_id/decisions", api.handlePostCaseDecisions)
	router.POST("/cases/:case_id/comments", api.handlePostCaseComment)
	router.POST("/cases/:case_id/case_tags", api.handlePostCaseTags)
	router.POST("/cases/:case_id/files", limits.RequestSizeLimiter(maxCaseFileSize), api.handlePostCaseFile)
	router.GET("/cases/files/:case_file_id/download_link", api.handleDownloadCaseFile)

	router.GET("/inboxes/:inbox_id", api.handleGetInboxById)
	router.PATCH("/inboxes/:inbox_id", api.handlePatchInbox)
	router.DELETE("/inboxes/:inbox_id", api.handleDeleteInbox)
	router.GET("/inboxes", api.handleListInboxes)
	router.POST("/inboxes", api.handlePostInbox)
	router.GET("/inbox_users", api.handleListAllInboxUsers)
	router.GET("/inbox_users/:inbox_user_id", api.handleGetInboxUserById)
	router.PATCH("/inbox_users/:inbox_user_id", api.handlePatchInboxUser)
	router.DELETE("/inbox_users/:inbox_user_id", api.handleDeleteInboxUser)
	router.GET("/inboxes/:inbox_id/users", api.handleListInboxUsers)
	router.POST("/inboxes/:inbox_id/users", api.handlePostInboxUser)

	router.GET("/tags", api.handleListTags)
	router.POST("/tags", api.handlePostTag)
	router.GET("/tags/:tag_id", api.handleGetTag)
	router.PATCH("/tags/:tag_id", api.handlePatchTag)
	router.DELETE("/tags/:tag_id", api.handleDeleteTag)

	router.GET("/data-model", api.GetDataModel)
	router.POST("/data-model/tables", api.CreateTable)
	router.PATCH("/data-model/tables/:tableID", api.UpdateDataModelTable)
	router.POST("/data-model/links", api.CreateLink)
	router.POST("/data-model/tables/:tableID/fields", api.CreateField)
	router.PATCH("/data-model/fields/:fieldID", api.UpdateDataModelField)
	router.DELETE("/data-model", api.DeleteDataModel)
	router.GET("/data-model/openapi", api.OpenAPI)
	router.POST("/data-model/pivots", api.createDataModelPivot)
	router.GET("/data-model/pivots", api.listDataModelPivots)

	router.POST("/transfers", api.handleCreateTransfer)
	router.GET("/transfers", api.handleQueryTransfers)
	router.PATCH("/transfers/:transfer_id", api.handleUpdateTransfer)
	router.GET("/transfers/:transfer_id", api.handleGetTransfer)
	router.POST("/transfers/:transfer_id/score", api.handleScoreTransfer)

	router.POST("/transfer/alerts", api.handleCreateTransferAlert)
	router.GET("/transfer/sent/alerts/:alert_id", api.handleGetTransferAlertSender)
	router.GET("/transfer/received/alerts/:alert_id", api.handleGetTransferAlertBeneficiary)
	router.GET("/transfer/sent/alerts", api.handleListTransferAlertsSender)
	router.GET("/transfer/received/alerts", api.handleListTransferAlertsBeneficiary)
	router.PATCH("/transfer/sent/alerts/:alert_id", api.handleUpdateTransferAlertSender)
	router.PATCH("/transfer/received/alerts/:alert_id",
		api.handleUpdateTransferAlertBeneficiary)

	router.GET("/licenses", api.handleListLicenses)
	router.POST("/licenses", api.handleCreateLicense)
	router.PATCH("/licenses/:license_id", api.handleUpdateLicense)
	router.GET("/licenses/:license_id", api.handleGetLicenseById)

	router.GET("/webhooks", api.handleListWebhooks)
	router.POST("/webhooks", api.handleRegisterWebhook)
	router.DELETE("/webhooks/:webhook_id", api.handleDeleteWebhook)
}
