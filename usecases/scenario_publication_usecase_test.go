package usecases

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

type ScenarioPublicationUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity                *mocks.EnforceSecurity
	executorFactory                *mocks.ExecutorFactory
	ingestedDataIndexesRepository  *mocks.IngestedDataIndexesRepository
	scenarioFetcher                *mocks.ScenarioFetcher
	scenarioListRepository         *mocks.ScenarioListRepository
	scenarioPublicationsRepository *mocks.ScenarioPublicationRepository
	scenarioPublisher              *mocks.ScenarioPublisher
	transaction                    *mocks.Executor
	transactionFactory             *mocks.TransactionFactory

	organizationId      string
	scenarioId          string
	iterationId         string
	publicationId       string
	scenarioPublication models.ScenarioPublication
	scenario            models.Scenario

	repositoryError error
	securityError   error
	ctx             context.Context
}

func (suite *ScenarioPublicationUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.ingestedDataIndexesRepository = new(mocks.IngestedDataIndexesRepository)
	suite.scenarioFetcher = new(mocks.ScenarioFetcher)
	suite.scenarioListRepository = new(mocks.ScenarioListRepository)
	suite.scenarioPublicationsRepository = new(mocks.ScenarioPublicationRepository)
	suite.scenarioPublisher = new(mocks.ScenarioPublisher)
	suite.transaction = new(mocks.Executor)
	suite.transactionFactory = &mocks.TransactionFactory{ExecMock: suite.transaction}

	suite.organizationId = "organizationId"
	suite.scenarioId = "scenarioId"
	suite.iterationId = "iterationId"
	suite.publicationId = "publicationId"
	suite.scenarioPublication = models.ScenarioPublication{
		Id:                  suite.publicationId,
		OrganizationId:      suite.organizationId,
		ScenarioId:          suite.scenarioId,
		ScenarioIterationId: suite.iterationId,
		PublicationAction:   models.Publish,
	}
	suite.scenario = models.Scenario{
		Id:             suite.scenarioId,
		OrganizationId: suite.organizationId,
	}

	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
	suite.ctx = context.Background()
}

func (suite *ScenarioPublicationUsecaseTestSuite) makeUsecase() *ScenarioPublicationUsecase {
	return &ScenarioPublicationUsecase{
		enforceSecurity:               suite.enforceSecurity,
		executorFactory:               suite.executorFactory,
		ingestedDataIndexesRepository: suite.ingestedDataIndexesRepository,
		OrganizationIdOfContext: func() (string, error) {
			return suite.organizationId, nil
		},
		scenarioFetcher:                suite.scenarioFetcher,
		scenarioListRepository:         suite.scenarioListRepository,
		scenarioPublicationsRepository: suite.scenarioPublicationsRepository,
		scenarioPublisher:              suite.scenarioPublisher,
		transactionFactory:             suite.transactionFactory,
	}
}

func (suite *ScenarioPublicationUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.executorFactory.AssertExpectations(t)
	suite.ingestedDataIndexesRepository.AssertExpectations(t)
	suite.scenarioFetcher.AssertExpectations(t)
	suite.scenarioListRepository.AssertExpectations(t)
	suite.scenarioPublicationsRepository.AssertExpectations(t)
	suite.scenarioPublisher.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
	suite.transactionFactory.AssertExpectations(t)
}

// GetScenarioPublication
func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetScenarioPublication_nominal() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.enforceSecurity.On("ReadScenarioPublication", suite.scenarioPublication).Return(nil)
	suite.scenarioPublicationsRepository.On(
		"GetScenarioPublicationById",
		suite.ctx,
		suite.transaction,
		suite.publicationId,
	).Return(suite.scenarioPublication, nil)

	publication, err := suite.makeUsecase().GetScenarioPublication(suite.ctx, suite.publicationId)

	suite.NoError(err)
	suite.Assert().NotEmpty(publication.Id)
	suite.Equal(suite.scenarioPublication, publication)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetScenarioPublication_get_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioPublicationsRepository.On(
		"GetScenarioPublicationById",
		suite.ctx,
		suite.transaction,
		suite.publicationId,
	).Return(models.ScenarioPublication{}, suite.repositoryError)

	publication, err := suite.makeUsecase().GetScenarioPublication(suite.ctx, suite.publicationId)

	suite.Equal(suite.repositoryError, err)
	suite.Assert().Empty(publication.Id)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetScenarioPublication_security_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioPublicationsRepository.On(
		"GetScenarioPublicationById",
		suite.ctx,
		suite.transaction,
		suite.publicationId,
	).Return(suite.scenarioPublication, nil)
	suite.enforceSecurity.On("ReadScenarioPublication", suite.scenarioPublication).Return(suite.securityError)

	publication, err := suite.makeUsecase().GetScenarioPublication(suite.ctx, suite.publicationId)

	suite.Equal(suite.securityError, err)
	suite.Assert().Empty(publication.Id)

	suite.AssertExpectations()
}

// ListScenarioPublications
func (suite *ScenarioPublicationUsecaseTestSuite) Test_ListScenarioPublications_nominal() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.enforceSecurity.On("ListScenarios", suite.organizationId).Return(nil)
	suite.scenarioPublicationsRepository.On(
		"ListScenarioPublicationsOfOrganization",
		suite.ctx,
		suite.transaction,
		suite.organizationId,
		models.ListScenarioPublicationsFilters{},
	).Return([]models.ScenarioPublication{suite.scenarioPublication}, nil)

	publications, err := suite.makeUsecase().ListScenarioPublications(suite.ctx,
		models.ListScenarioPublicationsFilters{})

	suite.NoError(err)
	suite.Assert().NotEmpty(publications)
	suite.Equal(suite.scenarioPublication, publications[0])

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ListScenarioPublications_security_error() {
	suite.enforceSecurity.On("ListScenarios", suite.organizationId).Return(suite.securityError)

	publications, err := suite.makeUsecase().ListScenarioPublications(suite.ctx,
		models.ListScenarioPublicationsFilters{})

	suite.Equal(suite.securityError, err)
	suite.Assert().Empty(publications)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ListScenarioPublications_get_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.enforceSecurity.On("ListScenarios", suite.organizationId).Return(nil)
	suite.scenarioPublicationsRepository.On(
		"ListScenarioPublicationsOfOrganization",
		suite.ctx,
		suite.transaction,
		suite.organizationId,
		models.ListScenarioPublicationsFilters{},
	).Return([]models.ScenarioPublication{}, suite.repositoryError)

	publications, err := suite.makeUsecase().ListScenarioPublications(suite.ctx,
		models.ListScenarioPublicationsFilters{})

	suite.Equal(suite.repositoryError, err)
	suite.Assert().Empty(publications)

	suite.AssertExpectations()
}

// ExecuteScenarioPublicationAction
func (suite *ScenarioPublicationUsecaseTestSuite) Test_ExecuteScenarioPublicationAction_nominal() {
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(models.ScenarioAndIteration{Scenario: suite.scenario}, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.scenarioPublisher.On("PublishOrUnpublishIteration", suite.ctx, suite.transaction, mock.Anything, models.Publish).
		Return([]models.ScenarioPublication{suite.scenarioPublication}, nil)

	publications, err := suite.makeUsecase().ExecuteScenarioPublicationAction(suite.ctx,
		models.PublishScenarioIterationInput{
			ScenarioIterationId: suite.iterationId,
			PublicationAction:   models.Publish,
		})

	suite.NoError(err)
	suite.Assert().NotEmpty(publications)
	suite.Equal(suite.scenarioPublication, publications[0])

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ExecuteScenarioPublicationAction_fetch_error() {
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(models.ScenarioAndIteration{}, suite.repositoryError)

	publications, err := suite.makeUsecase().ExecuteScenarioPublicationAction(suite.ctx,
		models.PublishScenarioIterationInput{
			ScenarioIterationId: suite.iterationId,
			PublicationAction:   models.Publish,
		})

	suite.Equal(suite.repositoryError, err)
	suite.Assert().Empty(publications)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_ExecuteScenarioPublicationAction_security_error() {
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(models.ScenarioAndIteration{Scenario: suite.scenario}, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(suite.securityError)

	publications, err := suite.makeUsecase().ExecuteScenarioPublicationAction(suite.ctx,
		models.PublishScenarioIterationInput{
			ScenarioIterationId: suite.iterationId,
			PublicationAction:   models.Publish,
		})

	suite.Equal(suite.securityError, err)
	suite.Assert().Empty(publications)

	suite.AssertExpectations()
}

func TestScenarioPublicationUsecase(t *testing.T) {
	suite.Run(t, new(ScenarioPublicationUsecaseTestSuite))
}
