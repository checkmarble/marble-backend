package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/decision_phantom"
	"github.com/checkmarble/marble-backend/usecases/decision_workflows"
	"github.com/checkmarble/marble-backend/usecases/inboxes"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/usecases/scheduled_execution"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transfers_data_read"
)

type UsecasesWithCreds struct {
	Usecaser
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
		enforceSecurity:               usecases.NewEnforceDecisionSecurity(),
		enforceSecurityScenario:       usecases.NewEnforceScenarioSecurity(),
		executorFactory:               usecases.NewExecutorFactory(),
		transactionFactory:            usecases.NewTransactionFactory(),
		ingestedDataReadRepository:    usecases.GetRepositories().IngestedDataReadRepository,
		dataModelRepository:           usecases.GetRepositories().DataModelRepository,
		repository:                    &usecases.GetRepositories().MarbleDbRepository,
		sanctionCheckConfigRepository: &usecases.GetRepositories().MarbleDbRepository,
		sanctionCheckUsecase:          usecases.NewSanctionCheckUsecase(),
		evaluateAstExpression:         usecases.NewEvaluateAstExpression(),
		decisionWorkflows:             usecases.NewDecisionWorkflows(),
		webhookEventsSender:           usecases.NewWebhookEventsUsecase(),
		snoozesReader:                 &usecases.GetRepositories().MarbleDbRepository,
		phantomUseCase: decision_phantom.NewPhantomDecisionUseCase(
			usecases.NewEnforcePhantomDecisionSecurity(), usecases.NewExecutorFactory(),
			usecases.GetRepositories().IngestedDataReadRepository,
			&usecases.GetRepositories().MarbleDbRepository, usecases.NewEvaluateAstExpression(),
			&usecases.GetRepositories().MarbleDbRepository, &usecases.GetRepositories().MarbleDbRepository,
			&usecases.GetRepositories().MarbleDbRepository, &usecases.GetRepositories().MarbleDbRepository,
			&usecases.GetRepositories().MarbleDbRepository),
		scenarioTestRunRepository: &usecases.GetRepositories().MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewSanctionCheckUsecase() SanctionCheckUsecaser {
	return SanctionCheckUsecase{
		enforceSecurityDecision: usecases.NewEnforceDecisionSecurity(),
		enforceSecurityCase:     usecases.NewEnforceCaseSecurity(),
		organizationRepository:  usecases.GetRepositories().OrganizationRepository,
		decisionRepository:      &usecases.GetRepositories().MarbleDbRepository,
		inboxRepository:         &usecases.GetRepositories().MarbleDbRepository,
		openSanctionsProvider:   usecases.GetRepositories().OpenSanctionsRepository,
		repository:              &usecases.GetRepositories().MarbleDbRepository,
		executorFactory:         usecases.NewExecutorFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewDecisionWorkflows() decision_workflows.DecisionsWorkflows {
	return decision_workflows.NewDecisionWorkflows(
		usecases.NewCaseUseCase(),
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewWebhookEventsUsecase(),
	)
}

func (usecases *UsecasesWithCreds) NewScenarioUsecase() ScenarioUsecase {
	return ScenarioUsecase{
		transactionFactory:  usecases.NewTransactionFactory(),
		scenarioFetcher:     usecases.NewScenarioFetcher(),
		validateScenarioAst: usecases.NewValidateScenarioAst(),
		executorFactory:     usecases.NewExecutorFactory(),
		enforceSecurity:     usecases.NewEnforceScenarioSecurity(),
		repository:          &usecases.GetRepositories().MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioIterationUsecase() ScenarioIterationUsecase {
	return ScenarioIterationUsecase{
		repository:                    &usecases.GetRepositories().MarbleDbRepository,
		sanctionCheckConfigRepository: &usecases.GetRepositories().MarbleDbRepository,
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
		repository:                &usecases.GetRepositories().MarbleDbRepository,
		scenarioFetcher:           usecases.NewScenarioFetcher(),
		transactionFactory:        usecases.NewTransactionFactory(),
		executorFactory:           usecases.NewExecutorFactory(),
		scenarioTestRunRepository: &usecases.GetRepositories().MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) AstExpressionUsecase() AstExpressionUsecase {
	return AstExpressionUsecase{
		EnforceSecurity:     usecases.NewEnforceScenarioSecurity(),
		DataModelRepository: usecases.GetRepositories().DataModelRepository,
		Repository:          &usecases.GetRepositories().MarbleDbRepository,
		executorFactory:     usecases.NewExecutorFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewCustomListUseCase() CustomListUseCase {
	return CustomListUseCase{
		enforceSecurity:      usecases.NewEnforceCustomListSecurity(),
		transactionFactory:   usecases.NewTransactionFactory(),
		executorFactory:      usecases.NewExecutorFactory(),
		CustomListRepository: usecases.GetRepositories().CustomListRepository,
	}
}

func (usecases *UsecasesWithCreds) NewScenarioPublicationUsecase() ScenarioPublicationUsecase {
	return ScenarioPublicationUsecase{
		transactionFactory:             usecases.NewTransactionFactory(),
		executorFactory:                usecases.NewExecutorFactory(),
		scenarioPublicationsRepository: usecases.GetRepositories().ScenarioPublicationRepository,
		enforceSecurity:                usecases.NewEnforceScenarioSecurity(),
		scenarioFetcher:                usecases.NewScenarioFetcher(),
		scenarioPublisher:              usecases.NewScenarioPublisher(),
		clientDbIndexEditor:            usecases.NewClientDbIndexEditor(),
	}
}

func (usecases *UsecasesWithCreds) NewClientDbIndexEditor() clientDbIndexEditor {
	return indexes.NewClientDbIndexEditor(
		usecases.NewExecutorFactory(),
		usecases.NewScenarioFetcher(),
		&usecases.GetRepositories().ClientDbRepository,
		usecases.NewEnforceScenarioSecurity(),
		usecases.NewEnforceOrganizationSecurity(),
	)
}

func (usecases *UsecasesWithCreds) NewOrganizationUseCase() OrganizationUseCase {
	return NewOrganizationUseCase(
		usecases.NewEnforceOrganizationSecurity(),
		usecases.NewTransactionFactory(),
		usecases.GetRepositories().OrganizationRepository,
		usecases.GetRepositories().DataModelRepository,
		usecases.GetRepositories().UserRepository,
		usecases.NewOrganizationCreator(),
		usecases.GetRepositories().OrganizationSchemaRepository,
		usecases.NewExecutorFactory(),
		*usecases.GetLicense(),
	)
}

func (usecases *UsecasesWithCreds) NewDataModelUseCase() DataModelUseCase {
	return DataModelUseCase{
		clientDbIndexEditor:          usecases.NewClientDbIndexEditor(),
		dataModelRepository:          usecases.GetRepositories().DataModelRepository,
		enforceSecurity:              usecases.NewEnforceOrganizationSecurity(),
		executorFactory:              usecases.NewExecutorFactory(),
		organizationSchemaRepository: usecases.GetRepositories().OrganizationSchemaRepository,
		transactionFactory:           usecases.NewTransactionFactory(),
	}
}

func (usecases *UsecasesWithCreds) NewIngestionUseCase() IngestionUseCase {
	return IngestionUseCase{
		enforceSecurity:       usecases.NewEnforceIngestionSecurity(),
		transactionFactory:    usecases.NewTransactionFactory(),
		executorFactory:       usecases.NewExecutorFactory(),
		ingestionRepository:   usecases.GetRepositories().IngestionRepository,
		blobRepository:        usecases.GetRepositories().BlobRepository,
		dataModelRepository:   usecases.GetRepositories().DataModelRepository,
		uploadLogRepository:   usecases.GetRepositories().UploadLogRepository,
		ingestionBucketUrl:    usecases.GetIngestionBucketUrl(),
		batchIngestionMaxSize: usecases.GetBatchIngestionMaxSize(),
	}
}

func (usecases *UsecasesWithCreds) NewRunScheduledExecution() scheduled_execution.RunScheduledExecution {
	return *scheduled_execution.NewRunScheduledExecution(
		&usecases.GetRepositories().MarbleDbRepository,
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.GetRepositories().IngestedDataReadRepository,
		usecases.NewTransactionFactory(),
		usecases.GetRepositories().TaskQueueRepository,
		usecases.GetRepositories().ScenarioPublicationRepository,
	)
}

func (usecases *UsecasesWithCreds) NewScheduledExecutionUsecase() ScheduledExecutionUsecase {
	return ScheduledExecutionUsecase{
		enforceSecurity:         usecases.NewEnforceDecisionSecurity(),
		transactionFactory:      usecases.NewTransactionFactory(),
		executorFactory:         usecases.NewExecutorFactory(),
		repository:              &usecases.GetRepositories().MarbleDbRepository,
		exportScheduleExecution: usecases.NewExportScheduleExecution(),
	}
}

func (usecases *UsecasesWithCreds) NewUserUseCase() UserUseCase {
	return UserUseCase{
		enforceUserSecurity: usecases.NewEnforceUserSecurity(),
		executorFactory:     usecases.NewExecutorFactory(),
		transactionFactory:  usecases.NewTransactionFactory(),
		userRepository:      usecases.GetRepositories().UserRepository,
	}
}

func (usecases *UsecasesWithCreds) NewCaseUseCase() *CaseUseCase {
	sec := security.EnforceSecurityInboxes{
		EnforceSecurity: usecases.NewEnforceSecurity(),
		Credentials:     usecases.Credentials,
	}
	return &CaseUseCase{
		enforceSecurity:    usecases.NewEnforceCaseSecurity(),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		repository:         &usecases.GetRepositories().MarbleDbRepository,
		decisionRepository: &usecases.GetRepositories().MarbleDbRepository,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity: sec,
			InboxRepository: &usecases.GetRepositories().MarbleDbRepository,
			Credentials:     usecases.Credentials,
			ExecutorFactory: usecases.NewExecutorFactory(),
		},
		caseManagerBucketUrl: usecases.GetCaseManagerBucketUrl(),
		blobRepository:       usecases.GetRepositories().BlobRepository,
		webhookEventsUsecase: usecases.NewWebhookEventsUsecase(),
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
		inboxRepository:    &usecases.GetRepositories().MarbleDbRepository,
		userRepository:     usecases.GetRepositories().UserRepository,
		credentials:        usecases.Credentials,
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    executorFactory,
		inboxReader: inboxes.InboxReader{
			EnforceSecurity: sec,
			InboxRepository: &usecases.GetRepositories().MarbleDbRepository,
			Credentials:     usecases.Credentials,
			ExecutorFactory: executorFactory,
		},
		inboxUsers: inboxes.InboxUsers{
			EnforceSecurity:     sec,
			InboxUserRepository: &usecases.GetRepositories().MarbleDbRepository,
			Credentials:         usecases.Credentials,
			TransactionFactory:  usecases.NewTransactionFactory(),
			ExecutorFactory:     executorFactory,
			UserRepository:      usecases.GetRepositories().UserRepository,
		},
	}
}

func (usecases *UsecasesWithCreds) NewTagUseCase() TagUseCase {
	return TagUseCase{
		enforceSecurity:    usecases.NewEnforceTagSecurity(),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		repository:         &usecases.GetRepositories().MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewApiKeyUseCase() ApiKeyUseCase {
	return ApiKeyUseCase{
		executorFactory: usecases.NewExecutorFactory(),
		enforceSecurity: &security.EnforceSecurityApiKeyImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		apiKeyRepository: &usecases.GetRepositories().MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewAnalyticsUseCase() AnalyticsUseCase {
	return AnalyticsUseCase{
		enforceSecurity: &security.EnforceSecurityAnalyticsImpl{
			EnforceSecurity: usecases.NewEnforceSecurity(),
			Credentials:     usecases.Credentials,
		},
		analyticsRepository: &usecases.GetRepositories().MarbleAnalyticsRepository,
	}
}

func (usecases *UsecasesWithCreds) NewTransferCheckUsecase() TransferCheckUsecase {
	return TransferCheckUsecase{
		dataModelRepository:               usecases.GetRepositories().DataModelRepository,
		decisionUseCase:                   usecases.NewDecisionUsecase(),
		decisionRepository:                &usecases.GetRepositories().MarbleDbRepository,
		enforceSecurity:                   security.NewEnforceSecurity(usecases.Credentials),
		executorFactory:                   usecases.NewExecutorFactory(),
		ingestionRepository:               usecases.GetRepositories().IngestionRepository,
		organizationRepository:            usecases.GetRepositories().OrganizationRepository,
		transactionFactory:                usecases.NewTransactionFactory(),
		transferMappingsRepository:        &usecases.GetRepositories().MarbleDbRepository,
		transferCheckEnrichmentRepository: usecases.GetRepositories().TransferCheckEnrichmentRepository,
		transferDataReader:                usecases.NewTransferDataReader(),
		partnersRepository:                usecases.GetRepositories().MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewTransferAlertsUsecase() TransferAlertsUsecase {
	return NewTransferAlertsUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.GetRepositories().OrganizationRepository,
		usecases.NewTransactionFactory(),
		&usecases.GetRepositories().MarbleDbRepository,
		&usecases.GetRepositories().MarbleDbRepository,
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewTransferDataReader(),
	)
}

func (usecases *UsecasesWithCreds) NewTransferDataReader() transfers_data_read.TransferDataReader {
	return transfers_data_read.NewTransferDataReader(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.GetRepositories().IngestedDataReadRepository,
		usecases.GetRepositories().DataModelRepository,
	)
}

func (usecases *UsecasesWithCreds) NewPartnerUsecase() PartnerUsecase {
	return PartnerUsecase{
		enforceSecurity:    security.NewEnforceSecurity(usecases.Credentials),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		partnersRepository: usecases.GetRepositories().MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewLicenseUsecase() ProtectedLicenseUseCase {
	return ProtectedLicenseUseCase{
		enforceSecurity:    security.NewEnforceSecurity(usecases.Credentials),
		transactionFactory: usecases.NewTransactionFactory(),
		executorFactory:    usecases.NewExecutorFactory(),
		licenseRepository:  &usecases.GetRepositories().MarbleDbRepository,
	}
}

func (usecases *UsecasesWithCreds) NewWebhookEventsUsecase() WebhookEventsUsecase {
	return NewWebhookEventsUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.GetRepositories().ConvoyRepository,
		usecases.GetRepositories().MarbleDbRepository,
		usecases.GetFailedWebhookRetryPageSize(),
		usecases.GetLicense().Webhooks,
		usecases.GetHasConvoyServerSetup(),
	)
}

func (usecases *UsecasesWithCreds) NewWebhooksUsecase() WebhooksUsecase {
	return NewWebhooksUsecase(
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.GetRepositories().ConvoyRepository,
	)
}

func (usecases *UsecasesWithCreds) NewRuleSnoozeUsecase() RuleSnoozeUsecase {
	return NewRuleSnoozeUsecase(
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.NewCaseUseCase(),
		&usecases.GetRepositories().MarbleDbRepository,
		&usecases.GetRepositories().MarbleDbRepository,
		&usecases.GetRepositories().MarbleDbRepository,
		security.NewEnforceSecurity(usecases.Credentials),
		usecases.NewWebhookEventsUsecase(),
	)
}

func (usecases UsecasesWithCreds) NewAsyncDecisionWorker() *scheduled_execution.AsyncDecisionWorker {
	w := scheduled_execution.NewAsyncDecisionWorker(
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.GetRepositories().ScenarioPublicationRepository,
		usecases.GetRepositories().DataModelRepository,
		usecases.GetRepositories().IngestedDataReadRepository,
		usecases.NewEvaluateAstExpression(),
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewTransactionFactory(),
		usecases.NewDecisionWorkflows(),
		usecases.NewWebhookEventsUsecase(),
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewScenarioFetcher(),
		&usecases.GetRepositories().MarbleDbRepository,
		decision_phantom.NewPhantomDecisionUseCase(
			usecases.NewEnforcePhantomDecisionSecurity(), usecases.NewExecutorFactory(),
			usecases.GetRepositories().IngestedDataReadRepository,
			&usecases.GetRepositories().MarbleDbRepository, usecases.NewEvaluateAstExpression(),
			&usecases.GetRepositories().MarbleDbRepository, &usecases.GetRepositories().MarbleDbRepository,
			&usecases.GetRepositories().MarbleDbRepository, &usecases.GetRepositories().MarbleDbRepository,
			&usecases.GetRepositories().MarbleDbRepository),
	)
	return &w
}

func (usecases UsecasesWithCreds) NewNewAsyncScheduledExecWorker() *scheduled_execution.AsyncScheduledExecWorker {
	w := scheduled_execution.NewAsyncScheduledExecWorker(
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.GetRepositories().ScenarioPublicationRepository,
		usecases.GetRepositories().DataModelRepository,
		usecases.GetRepositories().IngestedDataReadRepository,
		usecases.NewEvaluateAstExpression(),
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewDecisionWorkflows(),
		usecases.NewWebhookEventsUsecase(),
		&usecases.GetRepositories().MarbleDbRepository,
		usecases.NewScenarioFetcher(),
		&usecases.GetRepositories().MarbleDbRepository,
	)
	return &w
}
