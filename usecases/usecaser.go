package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/decision_workflows"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/scheduled_execution"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transfers_data_read"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type Usecaser interface {
	AstEvaluationEnvironmentFactory(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment
	GetBatchIngestionMaxSize() int
	GetCaseManagerBucketUrl() string
	GetFailedWebhookRetryPageSize() int
	GetHasConvoyServerSetup() bool
	GetIngestionBucketUrl() string
	GetLicense() *models.LicenseValidation
	GetRepositories() *repositories.Repositories
	NewAstValidator() scenarios.AstValidator
	NewEvaluateAstExpression() ast_eval.EvaluateAstExpression
	NewExecutorFactory() executor_factory.ExecutorFactory
	NewExportScheduleExecution() *scheduled_execution.ExportScheduleExecution
	NewLicenseUsecase() PublicLicenseUseCase
	NewLivenessUsecase() LivenessUsecase
	NewOrganizationCreator() organization.OrganizationCreator
	NewScenarioFetcher() scenarios.ScenarioFetcher
	NewScenarioPublisher() ScenarioPublisher
	NewSeedUseCase() SeedUseCase
	NewTaskQueueWorker(riverClient *river.Client[pgx.Tx]) *TaskQueueWorker
	NewTransactionFactory() executor_factory.TransactionFactory
	NewValidateScenarioAst() scenarios.ValidateScenarioAst
	NewValidateScenarioIteration() scenarios.ValidateScenarioIteration
}

type UsecaserWithCreds interface {
	AstExpressionUsecase() AstExpressionUsecase
	NewAnalyticsUseCase() AnalyticsUseCase
	NewApiKeyUseCase() ApiKeyUseCase
	NewAsyncDecisionWorker() *scheduled_execution.AsyncDecisionWorker
	NewCaseUseCase() *CaseUseCase
	NewClientDbIndexEditor() clientDbIndexEditor
	NewCustomListUseCase() CustomListUseCase
	NewDataModelUseCase() DataModelUseCase
	NewDecisionUsecase() DecisionUsecase
	NewDecisionWorkflows() decision_workflows.DecisionsWorkflows
	NewEnforceCaseSecurity() security.EnforceSecurityCase
	NewEnforceCustomListSecurity() security.EnforceSecurityCustomList
	NewEnforceDecisionSecurity() security.EnforceSecurityDecision
	NewEnforceIngestionSecurity() security.EnforceSecurityIngestion
	NewEnforceOrganizationSecurity() security.EnforceSecurityOrganization
	NewEnforcePhantomDecisionSecurity() security.EnforceSecurityPhantomDecision
	NewEnforceScenarioSecurity() security.EnforceSecurityScenario
	NewEnforceSecurity() security.EnforceSecurity
	NewEnforceTagSecurity() security.EnforceSecurityTags
	NewEnforceTestRunScenarioSecurity() security.EnforceSecurityTestRun
	NewEnforceUserSecurity() security.EnforceSecurityUser
	NewInboxUsecase() InboxUsecase
	NewIngestionUseCase() IngestionUseCase
	NewLicenseUsecase() ProtectedLicenseUseCase
	NewNewAsyncScheduledExecWorker() *scheduled_execution.AsyncScheduledExecWorker
	NewOrganizationUseCase() OrganizationUseCase
	NewPartnerUsecase() PartnerUsecase
	NewRuleSnoozeUsecase() RuleSnoozeUsecase
	NewRuleUsecase() RuleUsecase
	NewRunScheduledExecution() scheduled_execution.RunScheduledExecution
	NewSanctionCheckUsecase() SanctionCheckUsecase
	NewScenarioIterationUsecase() ScenarioIterationUsecase
	NewScenarioPublicationUsecase() ScenarioPublicationUsecase
	NewScenarioTestRunUseCase() ScenarioTestRunUsecase
	NewScenarioUsecase() ScenarioUsecase
	NewScheduledExecutionUsecase() ScheduledExecutionUsecase
	NewTagUseCase() TagUseCase
	NewTransferAlertsUsecase() TransferAlertsUsecase
	NewTransferCheckUsecase() TransferCheckUsecase
	NewTransferDataReader() transfers_data_read.TransferDataReader
	NewUserUseCase() UserUseCase
	NewWebhookEventsUsecase() WebhookEventsUsecase
	NewWebhooksUsecase() WebhooksUsecase
}
