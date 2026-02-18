package usecases

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ai_agent"
	"github.com/checkmarble/marble-backend/usecases/continuous_screening"
	"github.com/checkmarble/marble-backend/usecases/decision_phantom"
	"github.com/checkmarble/marble-backend/usecases/decision_workflows"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transfers_data_read"
	"github.com/checkmarble/marble-backend/usecases/webhooks"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"
	"github.com/checkmarble/marble-backend/utils"
)

type UsecasesWithCreds struct {
	Usecases
	Credentials models.Credentials
}

type UsecaseTransactionWrapper func(tx repositories.Transaction, org models.Organization, user models.User) *UsecasesWithCreds

// Used to recreate the whole usecase hierarchy from a transaction for Marble
// database and and organization for the ingested data database, impersonating
// the provided user.
func (usecases *UsecasesWithCreds) NewWithRootImpersonatedExecutor(tx repositories.Transaction,
	org models.Organization, user models.User,
) *UsecasesWithCreds {
	executorFactory := executor_factory.NewIdentityExecutorFactory(tx,
		usecases.Repositories.ExecutorGetter, org)

	return &UsecasesWithCreds{
		Usecases: usecases.Usecases.WithRootExecutor(executorFactory),
		Credentials: models.Credentials{
			OrganizationId: org.Id,
			Role:           usecases.Credentials.Role,
			ActorIdentity: models.Identity{
				UserId: user.UserId,
			},
		},
	}
}

func (usecases *UsecasesWithCreds) NewEnforceSecurity() security.EnforceSecurity {
	return &security.EnforceSecurityImpl{
		Credentials: usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceScenarioSecurity() security.EnforceSecurityScenario {
	return &security.EnforceSecurityScenarioImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceTestRunScenarioSecurity() security.EnforceSecurityTestRun {
	return &security.EnforceSecurotyTestRunImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceDecisionSecurity() security.EnforceSecurityDecision {
	return &security.EnforceSecurityDecisionImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforcePhantomDecisionSecurity() security.EnforceSecurityPhantomDecision {
	return &security.EnforceSecurityPhantomDecisionImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceCustomListSecurity() security.EnforceSecurityCustomList {
	return &security.EnforceSecurityCustomListImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceOrganizationSecurity() security.EnforceSecurityOrganization {
	return &security.EnforceSecurityOrganizationImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceIngestionSecurity() security.EnforceSecurityIngestion {
	return &security.EnforceSecurityIngestionImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceUserSecurity() security.EnforceSecurityUser {
	return &security.EnforceSecurityUserImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceCaseSecurity() security.EnforceSecurityCase {
	return &security.EnforceSecurityCaseImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceTagSecurity() security.EnforceSecurityTags {
	return &security.EnforceSecurityImpl{
		Credentials: usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceScreeningSecurity() security.EnforceSecurityScreening {
	return &security.EnforceSecurityImpl{
		Credentials: usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceAnnotationSecurity() security.EnforceSecurityAnnotation {
	return &security.EnforceSecurityAnnotationImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceSecurityContinuousScreening() security.EnforceSecurityContinuousScreening {
	return &security.EnforceSecurityContinuousScreeningImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewEnforceSecurityAudit() security.EnforceSecurityAudit {
	return &security.EnforceSecurityAuditImpl{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
}

func (usecases *UsecasesWithCreds) NewDecisionUsecase() DecisionUsecase {
	return DecisionUsecase{
		enforceSecurity:           usecases.NewEnforceDecisionSecurity(),
		enforceSecurityScenario:   usecases.NewEnforceScenarioSecurity(),
		executorFactory:           usecases.NewExecutorFactory(),
		transactionFactory:        usecases.NewTransactionFactory(),
		dataModelRepository:       usecases.Repositories.MarbleDbRepository,
		repository:                usecases.Repositories.MarbleDbRepository,
		screeningRepository:       usecases.Repositories.MarbleDbRepository,
		webhookEventsSender:       usecases.NewWebhookEventsUsecase(),
		phantomUseCase:            usecases.NewPhantomDecisionUseCase(),
		scenarioTestRunRepository: usecases.Repositories.MarbleDbRepository,
		scenarioEvaluator:         usecases.NewScenarioEvaluator(),
		openSanctionsRepository:   usecases.Repositories.OpenSanctionsRepository,
		taskQueueRepository:       usecases.Repositories.TaskQueueRepository,
		offloadedReader:           usecases.NewOffloadedReader(),
	}
}

func (usecases *UsecasesWithCreds) NewOffloadedReader() repositories.OffloadedReadWriter {
	return repositories.OffloadedReadWriter{
		Repository:          usecases.Repositories.MarbleDbRepository,
		BlobRepository:      usecases.Repositories.BlobRepository,
		OffloadingBucketUrl: usecases.offloadingBucketUrl,
	}
}

func (usecases *UsecasesWithCreds) NewPhantomDecisionUseCase() decision_phantom.PhantomDecisionUsecase {
	return decision_phantom.NewPhantomDecisionUseCase(
		usecases.NewEnforcePhantomDecisionSecurity(),
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewScenarioEvaluator(),
	)
}

func (usecases *UsecasesWithCreds) NewScenarioEvaluator() evaluate_scenario.ScenarioEvaluator {
	return evaluate_scenario.NewScenarioEvaluator(
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewScreeningUsecase(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.Repositories.IngestedDataReadRepository,
		usecases.NewEvaluateAstExpression(),
		usecases.Repositories.MarbleDbRepository,
		usecases.NewFeatureAccessReader(),
		usecases.Repositories.NameRecognitionRepository,
	)
}

func (usecases *UsecasesWithCreds) NewScreeningUsecase() ScreeningUsecase {
	return ScreeningUsecase{
		enforceSecurityScenario:   usecases.NewEnforceScenarioSecurity(),
		enforceSecurityDecision:   usecases.NewEnforceDecisionSecurity(),
		enforceSecurityCase:       usecases.NewEnforceCaseSecurity(),
		enforceSecurity:           usecases.NewEnforceScreeningSecurity(),
		featureAccessReader:       usecases.NewFeatureAccessReader(),
		externalRepository:        usecases.Repositories.MarbleDbRepository,
		caseUsecase:               usecases.NewCaseUseCase(),
		organizationRepository:    usecases.Repositories.MarbleDbRepository,
		inboxReader:               utils.Ptr(usecases.NewInboxReader()),
		scenarioFetcher:           usecases.NewScenarioFetcher(),
		openSanctionsProvider:     usecases.Repositories.OpenSanctionsRepository,
		screeningConfigRepository: usecases.Repositories.MarbleDbRepository,
		taskQueueRepository:       usecases.Repositories.TaskQueueRepository,
		repository:                usecases.Repositories.MarbleDbRepository,
		blobRepository:            usecases.Repositories.BlobRepository,
		blobBucketUrl:             usecases.caseManagerBucketUrl,
		executorFactory:           usecases.NewExecutorFactory(),
		transactionFactory:        usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewDecisionWorkflows() decision_workflows.DecisionsWorkflows {
	return decision_workflows.NewDecisionWorkflows(
		usecases.NewCaseUseCase(),
		usecases.Repositories.MarbleDbRepository,
		usecases.NewWebhookEventsUsecase(),
		usecases.NewScenarioEvaluator(),
		usecases.NewEvaluateAstExpression(),
		usecases.Repositories.TaskQueueRepository,
		usecases.caseManagerBucketUrl,
		utils.Ptr(usecases.NewAiAgentUsecase()),
	)
}

func (usecases *UsecasesWithCreds) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		transactionFactory:        usecases.NewTransactionFactory(),
		scenarioFetcher:           usecases.NewScenarioFetcher(),
		validateScenarioAst:       usecases.NewValidateScenarioAst(),
		executorFactory:           usecases.NewExecutorFactory(),
		enforceSecurity:           usecases.NewEnforceScenarioSecurity(),
		repository:                usecases.Repositories.MarbleDbRepository,
		workflowRepository:        usecases.Repositories.MarbleDbRepository,
		iterationRepository:       usecases.Repositories.MarbleDbRepository,
		screeningConfigRepository: usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewWorkflowUsecase() WorkflowUsecase {
	return WorkflowUsecase{
		executorFactory:     usecases.NewExecutorFactory(),
		enforceSecurity:     usecases.NewEnforceScenarioSecurity(),
		repository:          usecases.Repositories.MarbleDbRepository,
		scenarioRepository:  usecases.Repositories.MarbleDbRepository,
		validateScenarioAst: usecases.NewValidateScenarioAst(),
	}
}

func (usecases *UsecasesWithCreds) NewScenarioIterationUsecase() ScenarioIterationUsecase {
	return ScenarioIterationUsecase{
		repository:                usecases.Repositories.MarbleDbRepository,
		screeningConfigRepository: usecases.Repositories.MarbleDbRepository,
		enforceSecurity:           usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:           usecases.NewScenarioFetcher(),
		validateScenarioIteration: usecases.NewValidateScenarioIteration(),
		executorFactory:           usecases.NewExecutorFactory(),
		transactionFactory:        usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewRuleUsecase() RuleUsecase {
	return RuleUsecase{
		enforceSecurity:           usecases.NewEnforceScenarioSecurity(),
		repository:                usecases.Repositories.MarbleDbRepository,
		scenarioFetcher:           usecases.NewScenarioFetcher(),
		transactionFactory:        usecases.NewTransactionFactory(),
		executorFactory:           usecases.NewExecutorFactory(),
		scenarioTestRunRepository: usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return NewAstExpressionUsecase(
		usecases.NewExecutorFactory(),
		usecases.NewEnforceScenarioSecurity(),
		usecases.Repositories.MarbleDbRepository,
	)
}

func (usecases *UsecasesWithCreds) NewCustomListUseCase() CustomListUseCase {
	return CustomListUseCase{
		enforceSecurity:      usecases.NewEnforceCustomListSecurity(),
		transactionFactory:   usecases.NewTransactionFactory(),
		executorFactory:      usecases.NewExecutorFactory(),
		CustomListRepository: usecases.Repositories.CustomListRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioPublicationUsecase() *ScenarioPublicationUsecase {
	return NewScenarioPublicationUsecase(
		usecases.NewTransactionFactory(),
		usecases.NewExecutorFactory(),
		usecases.Repositories.ScenarioPublicationRepository,
		usecases.Repositories.TaskQueueRepository,
		usecases.NewEnforceScenarioSecurity(),
		usecases.NewScenarioFetcher(),
		usecases.NewScenarioPublisher(),
		usecases.NewClientDbIndexEditor(),
		usecases.NewFeatureAccessReader(),
		usecases.Repositories.OpenSanctionsRepository,
	)
}

func (usecases *UsecasesWithCreds) NewClientDbIndexEditor() indexes.ClientDbIndexEditor {
	return indexes.NewClientDbIndexEditor(
		usecases.NewExecutorFactory(),
		usecases.NewScenarioFetcher(),
		&usecases.Repositories.ClientDbRepository,
		usecases.NewEnforceScenarioSecurity(),
		usecases.NewEnforceOrganizationSecurity(),
	)
}

func (usecases *UsecasesWithCreds) NewOrganizationUseCase() OrganizationUseCase {
	return NewOrganizationUseCase(
		usecases.NewEnforceOrganizationSecurity(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewOrganizationCreator(),
		usecases.Repositories.OrganizationSchemaRepository,
		usecases.NewExecutorFactory(),
		usecases.NewFeatureAccessReader(),
	)
}

func (usecases *UsecasesWithCreds) NewDataModelUseCase() usecase {
	return usecase{
		clientDbIndexEditor:           usecases.NewClientDbIndexEditor(),
		dataModelRepository:           usecases.Repositories.MarbleDbRepository,
		enforceSecurity:               usecases.NewEnforceOrganizationSecurity(),
		executorFactory:               usecases.NewExecutorFactory(),
		organizationSchemaRepository:  usecases.Repositories.OrganizationSchemaRepository,
		transactionFactory:            usecases.NewTransactionFactory(),
		dataModelIngestedDataReadRepo: usecases.Repositories.IngestedDataReadRepository,
		indexEditor:                   usecases.NewClientDbIndexEditor(),
		taskQueueRepository:           usecases.Repositories.TaskQueueRepository,
	}
}

func (usecases *UsecasesWithCreds) NewAnalyticsSettingsUsecase() AnalyticsSettingsUsecase {
	return AnalyticsSettingsUsecase{
		enforceSecurity: usecases.NewEnforceOrganizationSecurity(),
		repository:      usecases.Repositories.MarbleDbRepository,
		executorFactory: usecases.NewExecutorFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewIngestionUseCase() IngestionUseCase {
	return IngestionUseCase{
		enforceSecurity:                     usecases.NewEnforceIngestionSecurity(),
		transactionFactory:                  usecases.NewTransactionFactory(),
		executorFactory:                     usecases.NewExecutorFactory(),
		ingestionRepository:                 usecases.Repositories.IngestionRepository,
		blobRepository:                      usecases.Repositories.BlobRepository,
		dataModelRepository:                 usecases.Repositories.MarbleDbRepository,
		uploadLogRepository:                 usecases.Repositories.UploadLogRepository,
		ingestionBucketUrl:                  usecases.ingestionBucketUrl,
		continuousScreeningRepository:       usecases.Repositories.MarbleDbRepository,
		continuousScreeningClientRepository: &usecases.Repositories.ClientDbRepository,
		batchIngestionMaxSize:               usecases.Usecases.batchIngestionMaxSize,
		taskEnqueuer:                        usecases.Repositories.TaskQueueRepository,
	}
}

func (usecases *UsecasesWithCreds) NewRunScheduledExecution() worker_jobs.RunScheduledExecution {
	return *worker_jobs.NewRunScheduledExecution(
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.Repositories.IngestedDataReadRepository,
		usecases.NewTransactionFactory(),
		usecases.Repositories.TaskQueueRepository,
		usecases.Repositories.ScenarioPublicationRepository,
	)
}

func (usecases *UsecasesWithCreds) NewScheduledExecutionUsecase() ScheduledExecutionUsecase {
	return ScheduledExecutionUsecase{
		enforceSecurity:     usecases.NewEnforceDecisionSecurity(),
		transactionFactory:  usecases.NewTransactionFactory(),
		executorFactory:     usecases.NewExecutorFactory(),
		repository:          usecases.Repositories.MarbleDbRepository,
		taskQueueRepository: usecases.Repositories.TaskQueueRepository,
	}
}

func (usecases *UsecasesWithCreds) NewUserUseCase() UserUseCase {
	return UserUseCase{
		enforceUserSecurity: usecases.NewEnforceUserSecurity(),
		executorFactory:     usecases.NewExecutorFactory(),
		transactionFactory:  usecases.NewTransactionFactory(),
		userRepository:      usecases.Repositories.MarbleDbRepository,
		firebaseAdmin:       usecases.firebaseAdmin,
	}
}

func (usecases *UsecasesWithCreds) NewInboxReader() inboxes.InboxReader {
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	return inboxes.InboxReader{
		EnforceSecurity: sec,
		InboxRepository: usecases.Repositories.MarbleDbRepository,
		Credentials:     usecases.Credentials,
		ExecutorFactory: usecases.NewExecutorFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewCaseUseCase() *CaseUseCase {
	return &CaseUseCase{
		enforceSecurity:         usecases.NewEnforceCaseSecurity(),
		enforceSecurityDecision: usecases.NewEnforceDecisionSecurity(),
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		repository:              usecases.Repositories.MarbleDbRepository,
		decisionRepository:      usecases.Repositories.MarbleDbRepository,
		inboxReader:             usecases.NewInboxReader(),
		caseManagerBucketUrl:    usecases.caseManagerBucketUrl,
		blobRepository:          usecases.Repositories.BlobRepository,
		webhookEventsUsecase:    usecases.NewWebhookEventsUsecase(),
		screeningRepository:     usecases.Repositories.MarbleDbRepository,
		ingestedDataReader:      usecases.NewIngestedDataReaderUsecase(),
		taskQueueRepository:     usecases.Repositories.TaskQueueRepository,
		featureAccessReader:     usecases.NewFeatureAccessReader(),
		aiAgentUsecase:          utils.Ptr(usecases.NewAiAgentUsecase()),
		publicApiAdapterUsecase: usecases.NewPublicApiAdapterUsecase(),
	}
}

func (usecases *UsecasesWithCreds) NewSuspiciousActivityReportUsecase() *SuspiciousActivityReportUsecase {
	return &SuspiciousActivityReportUsecase{
		executorFactory:      usecases.NewExecutorFactory(),
		transactionFactory:   usecases.NewTransactionFactory(),
		enforceCaseSecurity:  usecases.NewEnforceCaseSecurity(),
		caseUsecase:          usecases.NewCaseUseCase(),
		repository:           usecases.Repositories.MarbleDbRepository,
		blobRepository:       usecases.NewCaseUseCase().blobRepository,
		caseManagerBucketUrl: usecases.caseManagerBucketUrl,
	}
}

func (usecases *UsecasesWithCreds) NewInboxUsecase() InboxUsecase {
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	executorFactory := usecases.NewExecutorFactory()
	return InboxUsecase{
		enforceSecurity:    sec,
		inboxRepository:    usecases.Repositories.MarbleDbRepository,
		userRepository:     usecases.Repositories.MarbleDbRepository,
		credentials:        usecases.Credentials,
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    executorFactory,
		inboxReader:        usecases.NewInboxReader(),
		inboxUsers: inboxes.InboxUsers{
			EnforceSecurity:     sec,
			InboxUserRepository: usecases.Repositories.MarbleDbRepository,
			Credentials:         usecases.Credentials,
			TransactionFactory:  usecases.NewTransactionFactory(),
			ExecutorFactory:     executorFactory,
			UserRepository:      usecases.Repositories.MarbleDbRepository,
		},
	}
}

func (usecases *UsecasesWithCreds) NewTagUseCase() TagUseCase {
	return TagUseCase{
		enforceSecurity:    usecases.NewEnforceTagSecurity(),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		repository:         usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewApiKeyUseCase() ApiKeyUseCase {
	return ApiKeyUseCase{
		executorFactory: usecases.NewExecutorFactory(),
		enforceSecurity: &security.EnforceSecurityApiKeyImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		apiKeyRepository: usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewAnalyticsUseCase() AnalyticsUseCase {
	return AnalyticsUseCase{
		enforceSecurity: &security.EnforceSecurityAnalyticsImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		analyticsRepository: &usecases.Repositories.MarbleAnalyticsRepository,
	}
}

func (usecases *UsecasesWithCreds) NewTransferCheckUsecase() TransferCheckUsecase {
	return TransferCheckUsecase{
		dataModelRepository:               usecases.Repositories.MarbleDbRepository,
		decisionUseCase:                   usecases.NewDecisionUsecase(),
		decisionRepository:                usecases.Repositories.MarbleDbRepository,
		enforceSecurity:                   security.NewEnforceSecurity(usecases.Credentials),
		executorFactory:                   usecases.NewExecutorFactory(),
		ingestionRepository:               usecases.Repositories.IngestionRepository,
		organizationRepository:            usecases.Repositories.MarbleDbRepository,
		transactionFactory:                usecases.NewTransactionFactory(),
		transferMappingsRepository:        usecases.Repositories.MarbleDbRepository,
		transferCheckEnrichmentRepository: usecases.Repositories.TransferCheckEnrichmentRepository,
		transferDataReader:                usecases.NewTransferDataReader(),
		partnersRepository:                usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewTransferAlertsUsecase() TransferAlertsUsecase {
	return NewTransferAlertsUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewTransferDataReader(),
	)
}

func (usecases *UsecasesWithCreds) NewTransferDataReader() transfers_data_read.TransferDataReader {
	return transfers_data_read.NewTransferDataReader(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.Repositories.IngestedDataReadRepository,
		usecases.Repositories.MarbleDbRepository,
	)
}

func (usecases *UsecasesWithCreds) NewPartnerUsecase() PartnerUsecase {
	return PartnerUsecase{
		enforceSecurity:    security.NewEnforceSecurity(usecases.Credentials),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		partnersRepository: usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewLicenseUsecase() ProtectedLicenseUseCase {
	return ProtectedLicenseUseCase{
		enforceSecurity:    security.NewEnforceSecurity(usecases.Credentials),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		licenseRepository:  usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewWebhookEventsUsecase() webhooks.WebhookEventsUsecase {
	return webhooks.NewWebhookEventsUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.ConvoyRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.TaskQueueRepository,
		usecases.Usecases.failedWebhooksRetryPageSize,
		usecases.Usecases.license.Webhooks,
		usecases.Usecases.hasConvoyServerSetup,
		usecases.Usecases.webhookSystemMigrated,
		usecases.NewPublicApiAdapterUsecase(),
	)
}

func (usecases *UsecasesWithCreds) NewWebhooksUsecase() webhooks.WebhooksUsecase {
	return webhooks.NewWebhooksUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.ConvoyRepository,
		usecases.Repositories.MarbleDbRepository,
		webhooks.NewWebhookDeliveryService(webhooks.WebhookDeliveryConfig{
			AllowInsecureURLs: usecases.Usecases.allowInsecureWebhookURLs,
			MarbleVersion:     usecases.Usecases.apiVersion,
			IPWhitelist:       usecases.Usecases.webhookIPWhitelist,
		}),
		usecases.Usecases.webhookSystemMigrated,
	)
}

func (usecases *UsecasesWithCreds) NewRuleSnoozeUsecase() RuleSnoozeUsecase {
	return NewRuleSnoozeUsecase(
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.NewCaseUseCase(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewWebhookEventsUsecase(),
	)
}

func (usecases UsecasesWithCreds) NewAsyncDecisionWorker() *worker_jobs.AsyncDecisionWorker {
	w := worker_jobs.NewAsyncDecisionWorker(
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.IngestedDataReadRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewTransactionFactory(),
		usecases.NewOffloadedReader(),
		usecases.NewWebhookEventsUsecase(),
		usecases.NewScenarioFetcher(),
		usecases.NewPhantomDecisionUseCase(),
		usecases.NewScenarioEvaluator(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.TaskQueueRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewNewAsyncScheduledExecWorker() *worker_jobs.AsyncScheduledExecWorker {
	w := worker_jobs.NewAsyncScheduledExecWorker(
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
	)
	return &w
}

func (usecases UsecasesWithCreds) NewIndexCreationWorker() *worker_jobs.IndexCreationWorker {
	w := worker_jobs.NewIndexCreationWorker(
		usecases.NewExecutorFactory(),
		&usecases.Repositories.ClientDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewIndexCreationStatusWorker() *worker_jobs.IndexCreationStatusWorker {
	w := worker_jobs.NewIndexCreationStatusWorker(
		usecases.NewExecutorFactory(),
		&usecases.Repositories.ClientDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewIndexCleanupWorker() *worker_jobs.IndexCleanupWorker {
	w := worker_jobs.NewIndexCleanupWorker(
		usecases.NewExecutorFactory(),
		&usecases.Repositories.ClientDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewIndexDeletionWorker() *worker_jobs.IndexDeletionWorker {
	w := worker_jobs.NewIndexDeletionWorker(
		usecases.NewExecutorFactory(),
		&usecases.Repositories.ClientDbRepository,
		usecases.NewClientDbIndexEditor(),
	)
	return &w
}

func (usecases UsecasesWithCreds) NewTestRunSummaryWorker() *worker_jobs.TestRunSummaryWorker {
	w := worker_jobs.NewTestRunSummaryWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewMatchEnrichmentWorker() *worker_jobs.MatchEnrichmentWorker {
	w := worker_jobs.NewMatchEnrichmentWorker(
		usecases.NewExecutorFactory(),
		usecases.Usecases.Repositories.OpenSanctionsRepository,
		usecases.NewScreeningUsecase(),
		usecases.Repositories.MarbleDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewOffloadingWorker() *worker_jobs.OffloadingWorker {
	return worker_jobs.NewOffloadingWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.BlobRepository,
		usecases.offloadingBucketUrl,
		usecases.offloadingConfig,
	)
}

func (usecases UsecasesWithCreds) NewAutoAssignmentWorker() *worker_jobs.AutoAssignmentWorker {
	return worker_jobs.NewAutoAssignmentWorker(
		usecases.NewFeatureAccessReader(),
		usecases.Usecases.NewAutoAssignmentUsecase(),
	)
}

func (usecases UsecasesWithCreds) NewAnalyticsExportWorker() *worker_jobs.AnalyticsExportWorker {
	return worker_jobs.NewAnalyticsExportWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.NewAnalyticsExecutorFactory(),
		usecases.license,
		usecases.Repositories.MarbleDbRepository,
		usecases.analyticsConfig,
	)
}

func (usecases UsecasesWithCreds) NewAnalyticsMergeWorker() *worker_jobs.AnalyticsMergeWorker {
	return worker_jobs.NewAnalyticsMergeWorker(
		usecases.NewExecutorFactory(),
		usecases.NewAnalyticsExecutorFactory(),
		usecases.license,
		usecases.Repositories.MarbleDbRepository,
		usecases.analyticsConfig,
		usecases.Repositories.BlobRepository,
	)
}

func (usecases UsecasesWithCreds) NewIngestedDataReaderUsecase() IngestedDataReaderUsecase {
	return NewIngestedDataReaderUsecase(
		usecases.Repositories.IngestedDataReadRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.NewDataModelUseCase(),
	)
}

func (usecases UsecasesWithCreds) NewFeatureAccessReader() feature_access.FeatureAccessReader {
	return feature_access.NewFeatureAccessReader(
		usecases.NewEnforceOrganizationSecurity(),
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.Usecases.license,
		usecases.Usecases.hasConvoyServerSetup,
		usecases.Usecases.hasAnalyticsSetup,
		usecases.Usecases.webhookSystemMigrated,
		usecases.Usecases.hasOpensanctionsSetup,
		usecases.Usecases.hasNameRecognizerSetup,
	)
}

func (usecases *UsecasesWithCreds) NewScenarioTestRunUseCase() ScenarioTestRunUsecase {
	return ScenarioTestRunUsecase{
		transactionFactory:         usecases.NewTransactionFactory(),
		executorFactory:            usecases.NewExecutorFactory(),
		enforceSecurity:            usecases.NewEnforceTestRunScenarioSecurity(),
		repository:                 usecases.Repositories.MarbleDbRepository,
		clientDbIndexEditor:        usecases.NewClientDbIndexEditor(),
		scenarioRepository:         usecases.Repositories.MarbleDbRepository,
		scenarioIteratorRepository: usecases.Repositories.MarbleDbRepository,
		featureAccessReader:        usecases.NewFeatureAccessReader(),
		screeningConfigRepository:  usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewEntityAnnotationUsecase() EntityAnnotationUsecase {
	return EntityAnnotationUsecase{
		enforceSecurityAnnotation:  usecases.NewEnforceAnnotationSecurity(),
		repository:                 usecases.Repositories.MarbleDbRepository,
		dataModelRepository:        usecases.Repositories.MarbleDbRepository,
		ingestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
		caseUsecase:                usecases.NewCaseUseCase(),
		tagRepository:              usecases.Repositories.MarbleDbRepository,
		blobRepository:             usecases.Repositories.BlobRepository,
		bucketUrl:                  usecases.caseManagerBucketUrl,
		taskQueueRepository:        usecases.Repositories.TaskQueueRepository,
		executorFactory:            usecases.NewExecutorFactory(),
		transactionFactory:         usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewAiAgentUsecase() ai_agent.AiAgentUsecase {
	return ai_agent.NewAiAgentUsecase(
		usecases.NewEnforceCaseSecurity(),
		usecases.NewEnforceOrganizationSecurity(),
		usecases.Repositories.MarbleDbRepository,
		usecases.NewInboxReader(),
		usecases.NewExecutorFactory(),
		usecases.NewIngestedDataReaderUsecase(),
		usecases.NewDataModelUseCase(),
		utils.Ptr(usecases.NewRuleUsecase()),
		utils.Ptr(usecases.NewCustomListUseCase()),
		utils.Ptr(usecases.NewScenarioUsecase()),
		usecases.NewBillingUsecase(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.BlobRepository,
		usecases.Repositories.TaskQueueRepository,
		usecases.NewTransactionFactory(),
		usecases.NewFeatureAccessReader(),
		usecases.aiAgentConfig,
		usecases.caseManagerBucketUrl, // TODO: I think we could avoid passing the caseManagerBucketURL here only for the creation of the model
	)
}

func (usecases *UsecasesWithCreds) NewCaseReviewWorker(timeout time.Duration) *ai_agent.CaseReviewWorker {
	w := ai_agent.NewCaseReviewWorker(
		usecases.Repositories.BlobRepository,
		usecases.caseManagerBucketUrl,
		utils.Ptr(usecases.NewAiAgentUsecase()),
		usecases.NewExecutorFactory(),
		usecases.Repositories.MarbleDbRepository,
		timeout,
	)
	return &w
}

func (usecases *UsecasesWithCreds) NewUserSettingsUsecase() UserSettingsUsecase {
	return UserSettingsUsecase{
		executorFactory: usecases.NewExecutorFactory(),
		enforceSecurity: usecases.NewEnforceSecurity(),
		repository:      usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewDecisionWorkflowsWorker() *decision_workflows.DecisionWorkflowsWorker {
	return decision_workflows.NewDecisionWorkflowsWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.NewDecisionWorkflows(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.IngestedDataReadRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewWebhookEventsUsecase(),
	)
}

func (usecases *UsecasesWithCreds) NewAnalyticsQueryUsecase() AnalyticsQueryUsecase {
	return AnalyticsQueryUsecase{
		enforceSecurity:    usecases.NewEnforceScenarioSecurity(),
		executorFactory:    usecases.NewExecutorFactory(),
		analyticsFactory:   usecases.NewAnalyticsExecutorFactory(),
		license:            usecases.license,
		scenarioRepository: usecases.Repositories.MarbleDbRepository,
		inboxReader:        usecases.NewInboxReader(),
	}
}

func (usecases *UsecasesWithCreds) NewAnalyticsMetadataUsecase() AnalyticsMetadataUsecase {
	return AnalyticsMetadataUsecase{
		enforceSecurity:    usecases.NewEnforceScenarioSecurity(),
		executorFactory:    usecases.NewExecutorFactory(),
		analyticsFactory:   usecases.NewAnalyticsExecutorFactory(),
		scenarioRepository: usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewContinuousScreeningUsecase() *continuous_screening.ContinuousScreeningUsecase {
	return continuous_screening.NewContinuousScreeningUsecase(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.NewEnforceSecurityContinuousScreening(),
		usecases.NewEnforceCaseSecurity(),
		usecases.NewEnforceScreeningSecurity(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.TaskQueueRepository,
		&usecases.Repositories.ClientDbRepository,
		usecases.Repositories.OrganizationSchemaRepository,
		usecases.Repositories.IngestedDataReadRepository,
		utils.Ptr(usecases.NewIngestionUseCase()),
		usecases.Repositories.OpenSanctionsRepository,
		usecases.NewCaseUseCase(),
		utils.Ptr(usecases.NewInboxReader()),
		utils.Ptr(usecases.NewInboxUsecase()),
		usecases.NewFeatureAccessReader(),
		usecases.NewEntityAnnotationUsecase(),
	)
}

func (usecases *UsecasesWithCreds) NewContinuousScreeningDoScreeningWorker() *continuous_screening.DoScreeningWorker {
	return continuous_screening.NewDoScreeningWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.TaskQueueRepository,
		&usecases.Repositories.ClientDbRepository,
		usecases.Repositories.IngestedDataReadRepository,
		usecases.NewContinuousScreeningUsecase(),
	)
}

func (usecases *UsecasesWithCreds) NewContinuousScreeningApplyDeltaFileWorker() *continuous_screening.ApplyDeltaFileWorker {
	return continuous_screening.NewApplyDeltaFileWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.TaskQueueRepository,
		usecases.Repositories.BlobRepository,
		usecases.Repositories.OpenSanctionsRepository,
		usecases.continuousScreeningBucketUrl,
		usecases.NewContinuousScreeningUsecase(),
	)
}

func (usecases *UsecasesWithCreds) NewContinuousScreeningScanDatasetUpdatesWorker() *continuous_screening.ScanDatasetUpdatesWorker {
	return continuous_screening.NewScanDatasetUpdatesWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.OpenSanctionsRepository,
		usecases.Repositories.BlobRepository,
		usecases.Repositories.TaskQueueRepository,
		usecases.NewFeatureAccessReader(),
		usecases.continuousScreeningBucketUrl,
	)
}

func (usecases *UsecasesWithCreds) NewContinuousScreeningMatchEnrichmentWorker() *continuous_screening.ContinuousScreeningMatchEnrichmentWorker {
	return continuous_screening.NewContinuousScreeningMatchEnrichmentWorker(
		usecases.NewExecutorFactory(),
		usecases.Repositories.OpenSanctionsRepository,
		usecases.Repositories.MarbleDbRepository,
	)
}

func (usecases *UsecasesWithCreds) NewGenerateThumbnailWorker() *worker_jobs.GenerateThumbnailWorker {
	return worker_jobs.NewGenerateThumbnailWorker(
		usecases.Repositories.BlobRepository,
	)
}

func (usecases *UsecasesWithCreds) NewAuditUsecase() AuditUsecase {
	return NewAuditUsecase(
		usecases.NewEnforceSecurityAudit(),
		usecases.NewExecutorFactory(),
		usecases.license,
		usecases.Repositories.MarbleDbRepository,
	)
}

func (usecases UsecasesWithCreds) NewScheduledScenarioWorker() *worker_jobs.ScheduledScenarioWorker {
	runScheduledExecution := usecases.NewRunScheduledExecution()
	return worker_jobs.NewScheduledScenarioWorker(&runScheduledExecution)
}

func (usecases UsecasesWithCreds) NewScheduledExecutionWorker() *worker_jobs.ScheduledExecutionWorker {
	runScheduledExecution := usecases.NewRunScheduledExecution()
	return worker_jobs.NewScheduledExecutionWorker(&runScheduledExecution)
}

func (usecases UsecasesWithCreds) NewCsvIngestionWorker() *CsvIngestionWorker {
	ingestionUsecase := usecases.NewIngestionUseCase()
	return NewCsvIngestionWorker(&ingestionUsecase)
}

func (usecases UsecasesWithCreds) NewWebhookRetryWorker() *worker_jobs.WebhookRetryWorker {
	webhookEventsUsecase := usecases.NewWebhookEventsUsecase()
	return worker_jobs.NewWebhookRetryWorker(&webhookEventsUsecase)
}

func (usecases UsecasesWithCreds) NewWebhookDispatchWorker() *worker_jobs.WebhookDispatchWorker {
	return worker_jobs.NewWebhookDispatchWorker(
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.TaskQueueRepository,
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
	)
}

func (usecases UsecasesWithCreds) NewWebhookDeliveryWorker() *worker_jobs.WebhookDeliveryWorker {
	deliveryService := webhooks.NewWebhookDeliveryService(webhooks.WebhookDeliveryConfig{
		AllowInsecureURLs: usecases.Usecases.allowInsecureWebhookURLs,
		MarbleVersion:     usecases.Usecases.apiVersion,
		IPWhitelist:       usecases.Usecases.webhookIPWhitelist,
	})

	return worker_jobs.NewWebhookDeliveryWorker(
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.TaskQueueRepository,
		deliveryService,
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
	)
}

func (usecases UsecasesWithCreds) NewWebhookCleanupWorker() *worker_jobs.WebhookCleanupWorker {
	return worker_jobs.NewWebhookCleanupWorker(
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
	)
}

func (usecases *UsecasesWithCreds) NewDataModelDestroyUsecase() DataModelDestroyUsecase {
	return NewDataModelDestroyUsecase(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.NewEnforceOrganizationSecurity(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.OrganizationSchemaRepository,
	)
}

func (usecases *UsecasesWithCreds) NewPublicApiAdapterUsecase() PublicApiAdapterUsecase {
	return PublicApiAdapterUsecase{
		enforceSecurity: usecases.NewEnforceOrganizationSecurity(),
		repository:      usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewOrgImportUsecase() OrgImportUsecase {
	return NewOrgImportUsecase(
		usecases.NewWithRootImpersonatedExecutor,
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		security.EnforceSecurityOrgImportImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.OrganizationSchemaRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.firebaseAdmin,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewDataModelUseCase(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.CustomListRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewClientDbIndexEditor(),
		usecases.NewScenarioPublicationUsecase(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewIngestionUseCase(),
		usecases.NewDecisionUsecase(),
	)
}

func (usecases *UsecasesWithCreds) NewClient360Usecase() Client360Usecase {
	return NewClient360Usecase(
		usecases.NewEnforceSecurity(),
		usecases.NewExecutorFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.IngestedDataReadRepository,
		usecases.NewClientDbIndexEditor(),
	)
}
