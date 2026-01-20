package usecases

import (
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/usecases/auth"
	"github.com/checkmarble/marble-backend/usecases/billing"
	"github.com/checkmarble/marble-backend/usecases/continuous_screening"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/metrics_collection"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/worker_jobs"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

type Usecases struct {
	Repositories                 repositories.Repositories
	appName                      string
	apiVersion                   string
	batchIngestionMaxSize        int
	ingestionBucketUrl           string
	caseManagerBucketUrl         string
	offloadingBucketUrl          string
	offloadingConfig             infra.OffloadingConfig
	failedWebhooksRetryPageSize  int
	hasConvoyServerSetup         bool
	hasMetabaseSetup             bool
	hasOpensanctionsSetup        bool
	hasNameRecognizerSetup       bool
	license                      models.LicenseValidation
	metricsCollectionConfig      infra.MetricCollectionConfig
	firebaseAdmin                idp.Adminer
	aiAgentConfig                infra.AIAgentConfiguration
	analyticsConfig              infra.AnalyticsConfig
	continuousScreeningBucketUrl string
	marbleApiInternalUrl         string
	csCreateFullDatasetInterval  time.Duration
}

type Option func(*options)

func WithAppName(appName string) Option {
	return func(o *options) {
		o.appName = appName
	}
}

func WithApiVersion(apiVersion string) Option {
	return func(o *options) {
		o.apiVersion = apiVersion
	}
}

func WithIngestionBucketUrl(bucket string) Option {
	return func(o *options) {
		o.ingestionBucketUrl = bucket
	}
}

func WithOffloadingBucketUrl(bucket string) Option {
	return func(o *options) {
		o.offloadingBucketUrl = bucket
	}
}

func WithOffloading(cfg infra.OffloadingConfig) Option {
	return func(o *options) {
		o.offloadingConfig = cfg
	}
}

func WithCaseManagerBucketUrl(bucket string) Option {
	return func(o *options) {
		o.caseManagerBucketUrl = bucket
	}
}

func WithFailedWebhooksRetryPageSize(size int) Option {
	return func(o *options) {
		o.failedWebhooksRetryPageSize = size
	}
}

func WithLicense(license models.LicenseValidation) Option {
	return func(o *options) {
		o.license = license
	}
}

func WithBatchIngestionMaxSize(size int) Option {
	return func(o *options) {
		o.batchIngestionMaxSize = size
	}
}

func WithConvoyServer(url string) Option {
	return func(o *options) {
		if url != "" {
			o.hasConvoyServerSetup = true
		}
	}
}

func WithMetabase(url string) Option {
	return func(o *options) {
		if url != "" {
			o.hasMetabaseSetup = true
		}
	}
}

func WithOpensanctions(isSet bool) Option {
	return func(o *options) {
		if isSet {
			o.hasOpensanctionsSetup = true
		}
	}
}

func WithNameRecognition(isSet bool) Option {
	return func(o *options) {
		if isSet {
			o.hasNameRecognitionSetup = true
		}
	}
}

func WithFirebaseAdmin(provider auth.TokenProvider, client idp.Adminer) Option {
	return func(o *options) {
		if provider == auth.TokenProviderFirebase {
			o.firebaseClient = client
		}
	}
}

func WithAnalyticsConfig(config infra.AnalyticsConfig) Option {
	return func(o *options) {
		o.analyticsConfig = config
	}
}

func WithMetricsCollectionConfig(config infra.MetricCollectionConfig) Option {
	return func(o *options) {
		o.metricsCollectionConfig = config
	}
}

func WithAIAgentConfig(config infra.AIAgentConfiguration) Option {
	return func(o *options) {
		o.aiAgentConfig = config
	}
}

func WithContinuousScreeningBucketUrl(bucket string) Option {
	return func(o *options) {
		o.continuousScreeningBucketUrl = bucket
	}
}

func WithMarbleApiInternalUrl(url string) Option {
	return func(o *options) {
		o.marbleApiInternalUrl = url
	}
}

func WithCsCreateFullDatasetInterval(interval time.Duration) Option {
	return func(o *options) {
		o.csCreateFullDatasetInterval = interval
	}
}

type options struct {
	appName                      string
	apiVersion                   string
	batchIngestionMaxSize        int
	ingestionBucketUrl           string
	caseManagerBucketUrl         string
	offloadingBucketUrl          string
	offloadingConfig             infra.OffloadingConfig
	failedWebhooksRetryPageSize  int
	license                      models.LicenseValidation
	hasConvoyServerSetup         bool
	hasMetabaseSetup             bool
	hasOpensanctionsSetup        bool
	hasNameRecognitionSetup      bool
	metricsCollectionConfig      infra.MetricCollectionConfig
	firebaseClient               idp.Adminer
	aiAgentConfig                infra.AIAgentConfiguration
	analyticsConfig              infra.AnalyticsConfig
	continuousScreeningBucketUrl string
	marbleApiInternalUrl         string
	csCreateFullDatasetInterval  time.Duration
}

func newUsecasesWithOptions(repositories repositories.Repositories, o *options) Usecases {
	if o.batchIngestionMaxSize == 0 {
		o.batchIngestionMaxSize = DefaultApiBatchIngestionSize
	}
	return Usecases{
		Repositories:                 repositories,
		appName:                      o.appName,
		apiVersion:                   o.apiVersion,
		batchIngestionMaxSize:        o.batchIngestionMaxSize,
		ingestionBucketUrl:           o.ingestionBucketUrl,
		caseManagerBucketUrl:         o.caseManagerBucketUrl,
		offloadingBucketUrl:          o.offloadingBucketUrl,
		offloadingConfig:             o.offloadingConfig,
		failedWebhooksRetryPageSize:  o.failedWebhooksRetryPageSize,
		license:                      o.license,
		hasConvoyServerSetup:         o.hasConvoyServerSetup,
		hasMetabaseSetup:             o.hasMetabaseSetup,
		hasOpensanctionsSetup:        o.hasOpensanctionsSetup,
		hasNameRecognizerSetup:       o.hasNameRecognitionSetup,
		metricsCollectionConfig:      o.metricsCollectionConfig,
		firebaseAdmin:                o.firebaseClient,
		aiAgentConfig:                o.aiAgentConfig,
		analyticsConfig:              o.analyticsConfig,
		continuousScreeningBucketUrl: o.continuousScreeningBucketUrl,
		marbleApiInternalUrl:         o.marbleApiInternalUrl,
		csCreateFullDatasetInterval:  o.csCreateFullDatasetInterval,
	}
}

func NewUsecases(repositories repositories.Repositories, opts ...Option) Usecases {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return newUsecasesWithOptions(repositories, o)
}

func (usecases *Usecases) NewExecutorFactory() executor_factory.ExecutorFactory {
	return executor_factory.NewDbExecutorFactory(
		usecases.appName,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.ExecutorGetter,
	)
}

func (usecases *Usecases) NewTransactionFactory() executor_factory.TransactionFactory {
	return executor_factory.NewDbExecutorFactory(
		usecases.appName,
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.ExecutorGetter,
	)
}

func (usecases *Usecases) NewAnalyticsExecutorFactory() executor_factory.AnalyticsExecutorFactory {
	return executor_factory.NewAnalyticsExecutorFactory(usecases.analyticsConfig)
}

func (usecases *Usecases) NewVersionUsecase() VersionUsecase {
	return VersionUsecase{
		ApiVersion: usecases.apiVersion,
	}
}

func (usecases *Usecases) NewLivenessUsecase() LivenessUsecase {
	return LivenessUsecase{
		executorFactory:    usecases.NewExecutorFactory(),
		livenessRepository: usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *Usecases) NewHealthUsecase() HealthUsecase {
	return HealthUsecase{
		executorFactory:         usecases.NewExecutorFactory(),
		healthRepository:        usecases.Repositories.MarbleDbRepository,
		hasOpensanctionsSetup:   usecases.hasOpensanctionsSetup,
		openSanctionsRepository: &usecases.Repositories.OpenSanctionsRepository,
	}
}

func (usecases *Usecases) NewSeedUseCase() SeedUseCase {
	return SeedUseCase{
		transactionFactory:     usecases.NewTransactionFactory(),
		executorFactory:        usecases.NewExecutorFactory(),
		userRepository:         usecases.Repositories.MarbleDbRepository,
		organizationCreator:    usecases.NewOrganizationCreator(),
		organizationRepository: usecases.Repositories.MarbleDbRepository,
		customListRepository:   usecases.Repositories.CustomListRepository,
	}
}

func (usecases *Usecases) NewAllowedNetworksUsecase() AllowedNetworksUsecase {
	return AllowedNetworksUsecase{
		executorFactory: usecases.NewExecutorFactory(),
		repository:      usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *Usecases) NewOrganizationCreator() organization.OrganizationCreator {
	return organization.OrganizationCreator{
		CustomListRepository:   usecases.Repositories.CustomListRepository,
		ExecutorFactory:        usecases.NewExecutorFactory(),
		OrganizationRepository: usecases.Repositories.MarbleDbRepository,
		TransactionFactory:     usecases.NewTransactionFactory(),
	}
}

func (usecases *Usecases) AstEvaluationEnvironmentFactory(params ast_eval.EvaluationEnvironmentFactoryParams) ast_eval.AstEvaluationEnvironment {
	environment := ast_eval.NewAstEvaluationEnvironment()

	// execution of a scenario with a dedicated security context
	enforceSecurity := &security.EnforceSecurityImpl{
		Credentials: models.Credentials{
			OrganizationId: params.OrganizationId,
		},
	}

	environment.AddEvaluator(ast.FUNC_CUSTOM_LIST_ACCESS,
		evaluate.NewCustomListValuesAccess(
			usecases.Repositories.CustomListRepository,
			enforceSecurity,
			usecases.NewExecutorFactory(),
		),
	)

	environment.AddEvaluator(ast.FUNC_DB_ACCESS,
		evaluate.DatabaseAccess{
			OrganizationId:             params.OrganizationId,
			DataModel:                  params.DataModel,
			ClientObject:               params.ClientObject,
			ExecutorFactory:            usecases.NewExecutorFactory(),
			IngestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
			ReturnFakeValue:            params.DatabaseAccessReturnFakeValue,
		},
	)

	environment.AddEvaluator(ast.FUNC_PAYLOAD,
		evaluate.NewPayload(ast.FUNC_PAYLOAD, params.ClientObject))

	environment.AddEvaluator(ast.FUNC_AGGREGATOR, evaluate.AggregatorEvaluator{
		OrganizationId:             params.OrganizationId,
		DataModel:                  params.DataModel,
		ClientObject:               params.ClientObject,
		ExecutorFactory:            usecases.NewExecutorFactory(),
		IngestedDataReadRepository: usecases.Repositories.IngestedDataReadRepository,
		ReturnFakeValue:            params.DatabaseAccessReturnFakeValue,
	})

	environment.AddEvaluator(ast.FUNC_FILTER, evaluate.FilterEvaluator{
		DataModel: params.DataModel,
	})

	environment.AddEvaluator(ast.FUNC_TIMESTAMP_EXTRACT,
		evaluate.NewTimestampExtract(
			usecases.NewExecutorFactory(),
			usecases.Repositories.MarbleDbRepository,
			params.OrganizationId))

	return environment
}

func (usecases *Usecases) NewEvaluateAstExpression() ast_eval.EvaluateAstExpression {
	return ast_eval.EvaluateAstExpression{
		AstEvaluationEnvironmentFactory: usecases.AstEvaluationEnvironmentFactory,
	}
}

func (usecases *Usecases) NewScenarioPublisher() ScenarioPublisher {
	return scenarios.ScenarioPublisher{
		Repository:                     usecases.Repositories.MarbleDbRepository,
		ValidateScenarioIteration:      usecases.NewValidateScenarioIteration(),
		ScenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
		ScenarioTestRunRepository:      usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *Usecases) NewValidateScenarioAst() scenarios.ValidateScenarioAst {
	return &scenarios.ValidateScenarioAstImpl{
		AstValidator: usecases.NewAstValidator(),
	}
}

func (usecases *Usecases) NewValidateScenarioIteration() scenarios.ValidateScenarioIteration {
	return &scenarios.ValidateScenarioIterationImpl{
		AstValidator: usecases.NewAstValidator(),
	}
}

func (usecases *Usecases) NewAstValidator() scenarios.AstValidator {
	return &scenarios.AstValidatorImpl{
		DataModelRepository:             usecases.Repositories.MarbleDbRepository,
		AstEvaluationEnvironmentFactory: usecases.AstEvaluationEnvironmentFactory,
		ExecutorFactory:                 usecases.NewExecutorFactory(),
	}
}

func (usecases *Usecases) NewScenarioFetcher() scenarios.ScenarioFetcher {
	return scenarios.ScenarioFetcher{
		Repository: usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *Usecases) NewLicenseUsecase() PublicLicenseUseCase {
	return NewPublicLicenseUsecase(
		usecases.NewExecutorFactory(),
		usecases.Repositories.MetricsIngestionRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.license,
	)
}

func (usecases *Usecases) NewTaskQueueWorker(riverClient *river.Client[pgx.Tx], queueWhitelist []string) *TaskQueueWorker {
	return NewTaskQueueWorker(
		usecases.NewExecutorFactory(),
		usecases.Repositories.MarbleDbRepository,
		queueWhitelist,
		riverClient,
	)
}

func (usecases *Usecases) NewMetricsCollectionWorker(licenseConfiguration models.LicenseConfiguration) worker_jobs.MetricCollectionWorker {
	return worker_jobs.NewMetricCollectionWorker(
		metrics_collection.NewCollectorsV1(
			usecases.NewExecutorFactory(),
			usecases.Repositories.MarbleDbRepository,
			&usecases.Repositories.ClientDbRepository,
			usecases.apiVersion,
			licenseConfiguration,
		),
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
		usecases.metricsCollectionConfig,
	)
}

func (usecases *Usecases) NewMetricsIngestionUsecase() MetricsIngestionUsecase {
	return NewMetricsIngestionUsecase(
		usecases.Repositories.MetricsIngestionRepository,
		usecases.Repositories.MarbleDbRepository,
		usecases.NewExecutorFactory(),
	)
}

func (usecases *Usecases) NewAutoAssignmentUsecase() AutoAssignmentUsecase {
	return AutoAssignmentUsecase{
		executorFactory:    usecases.NewExecutorFactory(),
		transactionFactory: usecases.NewTransactionFactory(),
		caseRepository:     usecases.Repositories.MarbleDbRepository,
		orgRepository:      usecases.Repositories.MarbleDbRepository,
		repository:         usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *Usecases) NewOidcUsecase() OidcUsecase {
	return OidcUsecase{}
}

func (usecases *Usecases) NewBillingUsecase() billing.BillingUsecase {
	return billing.NewBillingUsecase(
		usecases.Repositories.LagoRepository.IsConfigured(),
		usecases.Repositories.LagoRepository,
		usecases.Repositories.TaskQueueRepository,
	)
}

func (usecases *Usecases) NewSendBillingEventWorker() *billing.SendLagoBillingEventWorker {
	return billing.NewSendLagoBillingEventWorker(
		usecases.Repositories.LagoRepository,
	)
}

func (usecases *Usecases) NewContinuousScreeningManifestUsecase() *continuous_screening.ContinuousScreeningManifestUsecase {
	return continuous_screening.NewContinuousScreeningManifestUsecase(
		usecases.NewExecutorFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.BlobRepository,
		usecases.marbleApiInternalUrl,
		usecases.continuousScreeningBucketUrl,
	)
}

func (usecases *Usecases) NewContinuousScreeningCreateFullDatasetWorker() *continuous_screening.CreateFullDatasetWorker {
	return continuous_screening.NewCreateFullDatasetWorker(
		usecases.NewExecutorFactory(),
		usecases.NewTransactionFactory(),
		usecases.Repositories.MarbleDbRepository,
		usecases.Repositories.IngestedDataReadRepository,
		usecases.Repositories.BlobRepository,
		usecases.continuousScreeningBucketUrl,
		usecases.csCreateFullDatasetInterval,
	)
}
