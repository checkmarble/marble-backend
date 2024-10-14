package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/scheduled_execution"
	"github.com/checkmarble/marble-backend/usecases/security"
)

type Usecases struct {
	Repositories                repositories.Repositories
	batchIngestionMaxSize       int
	ingestionBucketUrl          string
	caseManagerBucketUrl        string
	failedWebhooksRetryPageSize int
	license                     models.LicenseValidation
}

type Option func(*options)

func WithIngestionBucketUrl(bucket string) Option {
	return func(o *options) {
		o.ingestionBucketUrl = bucket
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

type options struct {
	batchIngestionMaxSize       int
	ingestionBucketUrl          string
	caseManagerBucketUrl        string
	failedWebhooksRetryPageSize int
	license                     models.LicenseValidation
}

func newUsecasesWithOptions(repositories repositories.Repositories, o *options) Usecases {
	if o.batchIngestionMaxSize == 0 {
		o.batchIngestionMaxSize = DefaultApiBatchIngestionSize
	}
	return Usecases{
		Repositories:                repositories,
		batchIngestionMaxSize:       o.batchIngestionMaxSize,
		ingestionBucketUrl:          o.ingestionBucketUrl,
		caseManagerBucketUrl:        o.caseManagerBucketUrl,
		failedWebhooksRetryPageSize: o.failedWebhooksRetryPageSize,
		license:                     o.license,
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
		usecases.Repositories.OrganizationRepository,
		usecases.Repositories.ExecutorGetter,
	)
}

func (usecases *Usecases) NewTransactionFactory() executor_factory.TransactionFactory {
	return executor_factory.NewDbExecutorFactory(
		usecases.Repositories.OrganizationRepository,
		usecases.Repositories.ExecutorGetter,
	)
}

func (usecases *Usecases) NewLivenessUsecase() LivenessUsecase {
	return LivenessUsecase{
		executorFactory:    usecases.NewExecutorFactory(),
		livenessRepository: &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecases *Usecases) NewSeedUseCase() SeedUseCase {
	return SeedUseCase{
		transactionFactory:     usecases.NewTransactionFactory(),
		executorFactory:        usecases.NewExecutorFactory(),
		userRepository:         usecases.Repositories.UserRepository,
		organizationCreator:    usecases.NewOrganizationCreator(),
		organizationRepository: usecases.Repositories.OrganizationRepository,
		customListRepository:   usecases.Repositories.CustomListRepository,
	}
}

func (usecases *Usecases) NewOrganizationCreator() organization.OrganizationCreator {
	return organization.OrganizationCreator{
		CustomListRepository:   usecases.Repositories.CustomListRepository,
		ExecutorFactory:        usecases.NewExecutorFactory(),
		OrganizationRepository: usecases.Repositories.OrganizationRepository,
		TransactionFactory:     usecases.NewTransactionFactory(),
	}
}

func (usecases *Usecases) NewExportScheduleExecution() *scheduled_execution.ExportScheduleExecution {
	return &scheduled_execution.ExportScheduleExecution{
		DecisionRepository:     &usecases.Repositories.MarbleDbRepository,
		OrganizationRepository: usecases.Repositories.OrganizationRepository,
		ExecutorFactory:        usecases.NewExecutorFactory(),
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

	return environment
}

func (usecases *Usecases) NewEvaluateAstExpression() ast_eval.EvaluateAstExpression {
	return ast_eval.EvaluateAstExpression{
		AstEvaluationEnvironmentFactory: usecases.AstEvaluationEnvironmentFactory,
	}
}

func (usecases *Usecases) NewScenarioPublisher() ScenarioPublisher {
	return scenarios.ScenarioPublisher{
		Repository:                     &usecases.Repositories.MarbleDbRepository,
		ValidateScenarioIteration:      usecases.NewValidateScenarioIteration(),
		ScenarioPublicationsRepository: usecases.Repositories.ScenarioPublicationRepository,
	}
}

func (usecases *Usecases) NewValidateScenarioIteration() scenarios.ValidateScenarioIteration {
	return &scenarios.ValidateScenarioIterationImpl{
		DataModelRepository:             usecases.Repositories.DataModelRepository,
		AstEvaluationEnvironmentFactory: usecases.AstEvaluationEnvironmentFactory,
		ExecutorFactory:                 usecases.NewExecutorFactory(),
	}
}

func (usecase *Usecases) NewScenarioFetcher() scenarios.ScenarioFetcher {
	return scenarios.ScenarioFetcher{
		Repository: &usecase.Repositories.MarbleDbRepository,
	}
}

func (usecases *Usecases) NewLicenseUsecase() PublicLicenseUseCase {
	return PublicLicenseUseCase{
		executorFactory:   usecases.NewExecutorFactory(),
		licenseRepository: &usecases.Repositories.MarbleDbRepository,
	}
}
