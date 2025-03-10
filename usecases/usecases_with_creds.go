package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/decision_phantom"
	"github.com/checkmarble/marble-backend/usecases/decision_workflows"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/feature_access"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/usecases/scheduled_execution"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transfers_data_read"
	"github.com/checkmarble/marble-backend/utils"
)

type UsecasesWithCreds struct {
	Usecases
	Credentials models.Credentials
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

func (usecases *UsecasesWithCreds) NewDecisionUsecase() DecisionUsecase {
	return DecisionUsecase{
		enforceSecurity:           usecases.NewEnforceDecisionSecurity(),
		enforceSecurityScenario:   usecases.NewEnforceScenarioSecurity(),
		executorFactory:           usecases.NewExecutorFactory(),
		transactionFactory:        usecases.NewTransactionFactory(),
		dataModelRepository:       usecases.Repositories.DataModelRepository,
		repository:                &usecases.Repositories.MarbleDbRepository,
		sanctionCheckRepository:   &usecases.Repositories.MarbleDbRepository,
		decisionWorkflows:         usecases.NewDecisionWorkflows(),
		webhookEventsSender:       usecases.NewWebhookEventsUsecase(),
		phantomUseCase:            usecases.NewPhantomDecisionUseCase(),
		scenarioTestRunRepository: &usecases.Repositories.MarbleDbRepository,
		scenarioEvaluator:         usecases.NewScenarioEvaluator(),
		featureAccessReader:       usecases.NewFeatureAccessReader(),
		openSanctionsRepository:   usecases.Repositories.OpenSanctionsRepository,
		taskQueueRepository:       usecases.Repositories.TaskQueueRepository,
	}
}

func (usecases *UsecasesWithCreds) NewPhantomDecisionUseCase() decision_phantom.PhantomDecisionUsecase {
	return decision_phantom.NewPhantomDecisionUseCase(
		usecases.NewEnforcePhantomDecisionSecurity(),
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewScenarioEvaluator(),
	)
}

func (usecases *UsecasesWithCreds) NewScenarioEvaluator() evaluate_scenario.ScenarioEvaluator {
	return evaluate_scenario.NewScenarioEvaluator(
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewSanctionCheckUsecase(),
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.Repositories.IngestedDataReadRepository,
		usecases.NewEvaluateAstExpression(),
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewFeatureAccessReader(),
		usecases.Repositories.NameRecognitionRepository,
	)
}

func (usecases *UsecasesWithCreds) NewSanctionCheckUsecase() SanctionCheckUsecase {
	return SanctionCheckUsecase{
		enforceSecurityScenario:       usecases.NewEnforceScenarioSecurity(),
		enforceSecurityDecision:       usecases.NewEnforceDecisionSecurity(),
		enforceSecurityCase:           usecases.NewEnforceCaseSecurity(),
		externalRepository:            &usecases.Repositories.MarbleDbRepository,
		organizationRepository:        usecases.Repositories.OrganizationRepository,
		inboxReader:                   utils.Ptr(usecases.NewInboxReader()),
		scenarioFetcher:               usecases.NewScenarioFetcher(),
		openSanctionsProvider:         usecases.Repositories.OpenSanctionsRepository,
		sanctionCheckConfigRepository: &usecases.Repositories.MarbleDbRepository,
		taskQueueRepository:           usecases.Repositories.TaskQueueRepository,
		repository:                    &usecases.Repositories.MarbleDbRepository,
		blobRepository:                usecases.Repositories.BlobRepository,
		blobBucketUrl:                 usecases.caseManagerBucketUrl,
		executorFactory:               usecases.NewExecutorFactory(),
		transactionFactory:            usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewDecisionWorkflows() decision_workflows.DecisionsWorkflows {
	return decision_workflows.NewDecisionWorkflows(
		usecases.NewCaseUseCase(),
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewWebhookEventsUsecase(),
		usecases.NewScenarioEvaluator(),
	)
}

func (usecases *UsecasesWithCreds) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		transactionFactory:  usecases.NewTransactionFactory(),
		scenarioFetcher:     usecases.NewScenarioFetcher(),
		validateScenarioAst: usecases.NewValidateScenarioAst(),
		executorFactory:     usecases.NewExecutorFactory(),
		enforceSecurity:     usecases.NewEnforceScenarioSecurity(),
		repository:          &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioIterationUsecase() ScenarioIterationUsecase {
	return ScenarioIterationUsecase{
		repository:                    &usecases.Repositories.MarbleDbRepository,
		sanctionCheckConfigRepository: &usecases.Repositories.MarbleDbRepository,
		enforceSecurity:               usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:               usecases.NewScenarioFetcher(),
		validateScenarioIteration:     usecases.NewValidateScenarioIteration(),
		executorFactory:               usecases.NewExecutorFactory(),
		transactionFactory:            usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewRuleUsecase() RuleUsecase {
	return RuleUsecase{
		enforceSecurity:           usecases.NewEnforceScenarioSecurity(),
		repository:                &usecases.Repositories.MarbleDbRepository,
		scenarioFetcher:           usecases.NewScenarioFetcher(),
		transactionFactory:        usecases.NewTransactionFactory(),
		executorFactory:           usecases.NewExecutorFactory(),
		scenarioTestRunRepository: &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return AstExpressionUsecase{
		EnforceSecurity:     usecases.NewEnforceScenarioSecurity(),
		DataModelRepository: usecases.Repositories.DataModelRepository,
		Repository:          &usecases.Repositories.MarbleDbRepository,
		executorFactory:     usecases.NewExecutorFactory(),
	}
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

func (usecases *UsecasesWithCreds) NewClientDbIndexEditor() clientDbIndexEditor {
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
		usecases.Repositories.OrganizationRepository,
		usecases.Repositories.DataModelRepository,
		usecases.Repositories.UserRepository,
		usecases.NewOrganizationCreator(),
		usecases.Repositories.OrganizationSchemaRepository,
		usecases.NewExecutorFactory(),
		usecases.NewFeatureAccessReader(),
	)
}

func (usecases *UsecasesWithCreds) NewDataModelUseCase() DataModelUseCase {
	return DataModelUseCase{
		clientDbIndexEditor:          usecases.NewClientDbIndexEditor(),
		dataModelRepository:          usecases.Repositories.DataModelRepository,
		enforceSecurity:              usecases.NewEnforceOrganizationSecurity(),
		executorFactory:              usecases.NewExecutorFactory(),
		organizationSchemaRepository: usecases.Repositories.OrganizationSchemaRepository,
		transactionFactory:           usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewIngestionUseCase() IngestionUseCase {
	return IngestionUseCase{
		enforceSecurity:       usecases.NewEnforceIngestionSecurity(),
		transactionFactory:    usecases.NewTransactionFactory(),
		executorFactory:       usecases.NewExecutorFactory(),
		ingestionRepository:   usecases.Repositories.IngestionRepository,
		blobRepository:        usecases.Repositories.BlobRepository,
		dataModelRepository:   usecases.Repositories.DataModelRepository,
		uploadLogRepository:   usecases.Repositories.UploadLogRepository,
		ingestionBucketUrl:    usecases.ingestionBucketUrl,
		batchIngestionMaxSize: usecases.Usecases.batchIngestionMaxSize,
	}
}

func (usecases *UsecasesWithCreds) NewRunScheduledExecution() scheduled_execution.RunScheduledExecution {
	return *scheduled_execution.NewRunScheduledExecution(
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.Repositories.IngestedDataReadRepository,
		usecases.NewTransactionFactory(),
		usecases.Repositories.TaskQueueRepository,
		usecases.Repositories.ScenarioPublicationRepository,
	)
}

func (usecases *UsecasesWithCreds) NewScheduledExecutionUsecase() ScheduledExecutionUsecase {
	return ScheduledExecutionUsecase{
		enforceSecurity:         usecases.NewEnforceDecisionSecurity(),
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		repository:              &usecases.Repositories.MarbleDbRepository,
		exportScheduleExecution: usecases.NewExportScheduleExecution(),
	}
}

func (usecases *UsecasesWithCreds) NewUserUseCase() UserUseCase {
	return UserUseCase{
		enforceUserSecurity: usecases.NewEnforceUserSecurity(),
		executorFactory:     usecases.NewExecutorFactory(),
		transactionFactory:  usecases.NewTransactionFactory(),
		userRepository:      usecases.Repositories.UserRepository,
	}
}

func (usecases *UsecasesWithCreds) NewInboxReader() inboxes.InboxReader {
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	return inboxes.InboxReader{
		EnforceSecurity: sec,
		InboxRepository: &usecases.Repositories.MarbleDbRepository,
		Credentials:     usecases.Credentials,
		ExecutorFactory: usecases.NewExecutorFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewCaseUseCase() *CaseUseCase {
	return &CaseUseCase{
		enforceSecurity:         usecases.NewEnforceCaseSecurity(),
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		repository:              &usecases.Repositories.MarbleDbRepository,
		decisionRepository:      &usecases.Repositories.MarbleDbRepository,
		inboxReader:             usecases.NewInboxReader(),
		caseManagerBucketUrl:    usecases.caseManagerBucketUrl,
		blobRepository:          usecases.Repositories.BlobRepository,
		userRepository:          usecases.Repositories.UserRepository,
		webhookEventsUsecase:    usecases.NewWebhookEventsUsecase(),
		sanctionCheckRepository: &usecases.Repositories.MarbleDbRepository,
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
		inboxRepository:    &usecases.Repositories.MarbleDbRepository,
		userRepository:     usecases.Repositories.UserRepository,
		credentials:        usecases.Credentials,
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    executorFactory,
		inboxReader:        usecases.NewInboxReader(),
		inboxUsers: inboxes.InboxUsers{
			EnforceSecurity:     sec,
			InboxUserRepository: &usecases.Repositories.MarbleDbRepository,
			Credentials:         usecases.Credentials,
			TransactionFactory:  usecases.NewTransactionFactory(),
			ExecutorFactory:     executorFactory,
			UserRepository:      usecases.Repositories.UserRepository,
		},
	}
}

func (usecases *UsecasesWithCreds) NewTagUseCase() TagUseCase {
	return TagUseCase{
		enforceSecurity:    usecases.NewEnforceTagSecurity(),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		repository:         &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewApiKeyUseCase() ApiKeyUseCase {
	return ApiKeyUseCase{
		executorFactory: usecases.NewExecutorFactory(),
		enforceSecurity: &security.EnforceSecurityApiKeyImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		apiKeyRepository: &usecases.Repositories.MarbleDbRepository,
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
		dataModelRepository:               usecases.Repositories.DataModelRepository,
		decisionUseCase:                   usecases.NewDecisionUsecase(),
		decisionRepository:                &usecases.Repositories.MarbleDbRepository,
		enforceSecurity:                   security.NewEnforceSecurity(usecases.Credentials),
		executorFactory:                   usecases.NewExecutorFactory(),
		ingestionRepository:               usecases.Repositories.IngestionRepository,
		organizationRepository:            usecases.Repositories.OrganizationRepository,
		transactionFactory:                usecases.NewTransactionFactory(),
		transferMappingsRepository:        &usecases.Repositories.MarbleDbRepository,
		transferCheckEnrichmentRepository: usecases.Repositories.TransferCheckEnrichmentRepository,
		transferDataReader:                usecases.NewTransferDataReader(),
		partnersRepository:                usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewTransferAlertsUsecase() TransferAlertsUsecase {
	return NewTransferAlertsUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.Repositories.OrganizationRepository,
		usecases.NewTransactionFactory(),
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewTransferDataReader(),
	)
}

func (usecases *UsecasesWithCreds) NewTransferDataReader() transfers_data_read.TransferDataReader {
	return transfers_data_read.NewTransferDataReader(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.Repositories.IngestedDataReadRepository,
		usecases.Repositories.DataModelRepository,
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
		licenseRepository:  &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewWebhookEventsUsecase() WebhookEventsUsecase {
	return NewWebhookEventsUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.Repositories.ConvoyRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.Usecases.failedWebhooksRetryPageSize,
		usecases.Usecases.license.Webhooks,
		usecases.Usecases.hasConvoyServerSetup,
	)
}

func (usecases *UsecasesWithCreds) NewWebhooksUsecase() WebhooksUsecase {
	return NewWebhooksUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.ConvoyRepository,
	)
}

func (usecases *UsecasesWithCreds) NewRuleSnoozeUsecase() RuleSnoozeUsecase {
	return NewRuleSnoozeUsecase(
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.NewCaseUseCase(),
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		&usecases.Repositories.MarbleDbRepository,
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewWebhookEventsUsecase(),
	)
}

func (usecases UsecasesWithCreds) NewAsyncDecisionWorker() *scheduled_execution.AsyncDecisionWorker {
	w := scheduled_execution.NewAsyncDecisionWorker(
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.Repositories.DataModelRepository,
		usecases.Repositories.IngestedDataReadRepository,
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewTransactionFactory(),
		usecases.NewDecisionWorkflows(),
		usecases.NewWebhookEventsUsecase(),
		usecases.NewScenarioFetcher(),
		usecases.NewPhantomDecisionUseCase(),
		usecases.NewScenarioEvaluator(),
		&usecases.Repositories.MarbleDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewNewAsyncScheduledExecWorker() *scheduled_execution.AsyncScheduledExecWorker {
	w := scheduled_execution.NewAsyncScheduledExecWorker(
		&usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
	)
	return &w
}

func (usecases UsecasesWithCreds) NewIndexCreationWorker() *scheduled_execution.IndexCreationWorker {
	w := scheduled_execution.NewIndexCreationWorker(
		usecases.NewExecutorFactory(),
		&usecases.Repositories.ClientDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewIndexCreationStatusWorker() *scheduled_execution.IndexCreationStatusWorker {
	w := scheduled_execution.NewIndexCreationStatusWorker(
		usecases.NewExecutorFactory(),
		&usecases.Repositories.ClientDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewIndexCleanupWorker() *scheduled_execution.IndexCleanupWorker {
	w := scheduled_execution.NewIndexCleanupWorker(
		usecases.NewExecutorFactory(),
		&usecases.Repositories.ClientDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewTestRunSummaryWorker() *scheduled_execution.TestRunSummaryWorker {
	w := scheduled_execution.NewTestRunSummaryWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		&usecases.Repositories.MarbleDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewMatchEnrichmentWorker() *scheduled_execution.MatchEnrichmentWorker {
	w := scheduled_execution.NewMatchEnrichmentWorker(
		usecases.NewExecutorFactory(),
		usecases.NewSanctionCheckUsecase(),
		&usecases.Repositories.MarbleDbRepository,
	)
	return &w
}

func (usecases UsecasesWithCreds) NewIngestedDataReaderUsecase() IngestedDataReaderUsecase {
	return IngestedDataReaderUsecase{
		ingestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
		dataModelRepository:        usecases.Repositories.DataModelRepository,
		executorFactory:            usecases.NewExecutorFactory(),
	}
}

func (usecases UsecasesWithCreds) NewFeatureAccessReader() feature_access.FeatureAccessReader {
	return feature_access.NewFeatureAccessReader(
		usecases.NewEnforceOrganizationSecurity(),
		usecases.Repositories.OrganizationRepository,
		usecases.NewExecutorFactory(),
		usecases.Usecases.license,
		usecases.Usecases.hasConvoyServerSetup,
		usecases.Usecases.hasMetabaseSetup,
		usecases.Usecases.hasOpensanctionsSetup,
		usecases.Usecases.hasTestMode,
	)
}

func (usecases *UsecasesWithCreds) NewScenarioTestRunUseCase() ScenarioTestRunUsecase {
	return ScenarioTestRunUsecase{
		transactionFactory:            usecases.NewTransactionFactory(),
		executorFactory:               usecases.NewExecutorFactory(),
		enforceSecurity:               usecases.NewEnforceTestRunScenarioSecurity(),
		repository:                    &usecases.Repositories.MarbleDbRepository,
		clientDbIndexEditor:           usecases.NewClientDbIndexEditor(),
		scenarioRepository:            &usecases.Repositories.MarbleDbRepository,
		featureAccessReader:           usecases.NewFeatureAccessReader(),
		sanctionCheckConfigRepository: &usecases.Repositories.MarbleDbRepository,
	}
}
