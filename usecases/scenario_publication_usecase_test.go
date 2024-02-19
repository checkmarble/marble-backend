package usecases

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
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

	organizationId                string
	scenarioId                    string
	iterationId                   string
	publicationId                 string
	scenarioPublication           models.ScenarioPublication
	scenario                      models.Scenario
	scenarioIteration             models.ScenarioIteration
	scenarioAndIteration          models.ScenarioAndIteration
	scenarioAndIterationWithQuery models.ScenarioAndIteration
	existingIndexes               []models.ConcreteIndex

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
	suite.scenarioIteration = models.ScenarioIteration{
		Id:                            suite.iterationId,
		OrganizationId:                suite.organizationId,
		ScenarioId:                    suite.scenarioId,
		TriggerConditionAstExpression: &ast.Node{},
	}
	suite.scenarioAndIteration = models.ScenarioAndIteration{
		Scenario:  suite.scenario,
		Iteration: suite.scenarioIteration,
	}

	// setup an iteration that requires an index to be created
	suite.scenarioAndIterationWithQuery = models.ScenarioAndIteration{
		Scenario:  suite.scenario,
		Iteration: suite.scenarioIteration,
	}
	astJson := `{
		"name": "Or",
		"children": [
		  {
		    "name": "And",
		    "children": [
			{
			  "name": "\u003e",
			  "children": [
			    {
				"name": "Aggregator",
				"named_children": {
				  "aggregator": { "constant": "COUNT_DISTINCT" },
				  "fieldName": { "constant": "object_id" },
				  "filters": {
				    "name": "List",
				    "children": [
					{
					  "name": "Filter",
					  "named_children": {
					    "fieldName": { "constant": "new_field" },
					    "operator": { "constant": "=" },
					    "tableName": { "constant": "table" },
					    "value": { "constant": "dummy" }
					  }
					}
				    ]
				  },
				  "label": { "constant": "test" },
				  "tableName": { "constant": "table" }
				}
			    },
			    { "constant": 0 }
			  ]
			}
		    ]
		  }
		]
	    }`
	astNodeDto := dto.NodeDto{}
	err := json.Unmarshal([]byte(astJson), &astNodeDto)
	suite.Require().NoError(err)
	astNode, err := dto.AdaptASTNode(astNodeDto)
	suite.Require().NoError(err)
	suite.scenarioAndIterationWithQuery.Iteration.TriggerConditionAstExpression = &astNode
	suite.existingIndexes = []models.ConcreteIndex{
		{
			TableName: "table", Indexed: []models.FieldName{"a", "b"},
			Included: []models.FieldName{"c", "d"},
		},
	}

	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
	suite.ctx = utils.StoreLoggerInContext(context.Background(), utils.NewLogger("test"))
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
		Return(suite.scenarioAndIteration, nil)
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
		Return(suite.scenarioAndIteration, nil)
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

func (suite *ScenarioPublicationUsecaseTestSuite) Test_getIndexesToCreate_nominal_1() {
	// no preparation is currently running
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(0, nil)

	toCreate, numPending, err := suite.makeUsecase().getIndexesToCreate(suite.ctx, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Empty(toCreate)
	suite.Assert().Equal(0, numPending)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_getIndexesToCreate_nominal_2() {
	// another prepration is running
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(1, nil)

	toCreate, numPending, err := suite.makeUsecase().getIndexesToCreate(suite.ctx, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Empty(toCreate)
	suite.Assert().Equal(1, numPending)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_getIndexesToCreate_nominal_3() {
	// an index needs to be created
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(suite.scenarioAndIterationWithQuery, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(1, nil)

	toCreate, numPending, err := suite.makeUsecase().getIndexesToCreate(suite.ctx, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Equal(1, len(toCreate))
	suite.Assert().Equal(1, numPending)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_getIndexesToCreate_security_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(suite.securityError)

	_, _, err := suite.makeUsecase().getIndexesToCreate(suite.ctx, suite.iterationId)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetPublicationPreparationStatus_nominal_1() {
	// no preparation is currently running
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(0, nil)

	status, err := suite.makeUsecase().GetPublicationPreparationStatus(suite.ctx, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Equal(models.PublicationPreparationStatus{
		PreparationStatus:        models.PreparationStatusReadyToActivate,
		PreparationServiceStatus: models.PreparationServiceStatusAvailable,
	}, status)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_GetPublicationPreparationStatus_nominal_2() {
	// another prepration is running
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(1, nil)

	status, err := suite.makeUsecase().GetPublicationPreparationStatus(suite.ctx, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Equal(models.PublicationPreparationStatus{
		PreparationStatus:        models.PreparationStatusReadyToActivate,
		PreparationServiceStatus: models.PreparationServiceStatusOccupied,
	}, status)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_StartPublicationPreparation_none_to_create() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(0, nil)

	err := suite.makeUsecase().StartPublicationPreparation(suite.ctx, suite.iterationId)

	suite.NoError(err)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_StartPublicationPreparation_preparation_already_in_progress() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(suite.scenarioAndIterationWithQuery, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(1, nil)

	err := suite.makeUsecase().StartPublicationPreparation(suite.ctx, suite.iterationId)

	suite.Assert().ErrorIs(err, models.ConflictError)

	suite.AssertExpectations()
}

func (suite *ScenarioPublicationUsecaseTestSuite) Test_StartPublicationPreparation_preparation_nominal() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction, suite.iterationId).
		Return(suite.scenarioAndIterationWithQuery, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(0, nil)
	suite.ingestedDataIndexesRepository.On("CreateIndexesAsync", suite.ctx, suite.transaction, []models.ConcreteIndex{
		{
			TableName: "table",
			Indexed:   []models.FieldName{"new_field"},
			Included:  []models.FieldName{"object_id"},
		},
	}).Return(nil)

	err := suite.makeUsecase().StartPublicationPreparation(suite.ctx, suite.iterationId)

	suite.NoError(err)

	suite.AssertExpectations()
}

func TestScenarioPublicationUsecase(t *testing.T) {
	suite.Run(t, new(ScenarioPublicationUsecaseTestSuite))
}
