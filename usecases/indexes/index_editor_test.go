package indexes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

type ClientDbIndexEditorTestSuite struct {
	suite.Suite
	enforceSecurity               *mocks.EnforceSecurity
	enforceSecurityDataModel      *mocks.EnforceSecurity
	executorFactory               *mocks.ExecutorFactory
	ingestedDataIndexesRepository *mocks.IngestedDataIndexesRepository
	scenarioFetcher               *mocks.ScenarioFetcher
	transaction                   *mocks.Executor

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

func (suite *ClientDbIndexEditorTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.enforceSecurityDataModel = new(mocks.EnforceSecurity)
	suite.executorFactory = new(mocks.ExecutorFactory)
	suite.ingestedDataIndexesRepository = new(mocks.IngestedDataIndexesRepository)
	suite.scenarioFetcher = new(mocks.ScenarioFetcher)
	suite.transaction = new(mocks.Executor)

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
			TableName: "table", Indexed: []string{"a", "b"},
			Included: []string{"c", "d"},
		},
	}

	suite.repositoryError = errors.New("some repository error")
	suite.securityError = errors.New("some security error")
	suite.ctx = utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
}

func (suite *ClientDbIndexEditorTestSuite) makeUsecase() ClientDbIndexEditor {
	return NewClientDbIndexEditor(
		suite.executorFactory,
		suite.scenarioFetcher,
		suite.ingestedDataIndexesRepository,
		suite.enforceSecurity,
		suite.enforceSecurityDataModel,
		func() (string, error) {
			return suite.organizationId, nil
		},
	)
}

func (suite *ClientDbIndexEditorTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.executorFactory.AssertExpectations(t)
	suite.ingestedDataIndexesRepository.AssertExpectations(t)
	suite.scenarioFetcher.AssertExpectations(t)
	suite.transaction.AssertExpectations(t)
}

// GetIndexesToCreate
func (suite *ClientDbIndexEditorTestSuite) Test_GetIndexesToCreate_nominal_1() {
	// no preparation is currently running
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(0, nil)

	toCreate, numPending, err := suite.makeUsecase().GetIndexesToCreate(suite.ctx, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Empty(toCreate)
	suite.Assert().Equal(0, numPending)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_GetIndexesToCreate_nominal_2() {
	// another prepration is running
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(1, nil)

	toCreate, numPending, err := suite.makeUsecase().GetIndexesToCreate(suite.ctx, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Empty(toCreate)
	suite.Assert().Equal(1, numPending)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_GetIndexesToCreate_nominal_3() {
	// an index needs to be created
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(suite.scenarioAndIterationWithQuery, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(nil)
	suite.ingestedDataIndexesRepository.On("ListAllValidIndexes", suite.ctx, suite.transaction).
		Return(suite.existingIndexes, nil)
	suite.ingestedDataIndexesRepository.On("CountPendingIndexes", suite.ctx, suite.transaction).Return(1, nil)

	toCreate, numPending, err := suite.makeUsecase().GetIndexesToCreate(suite.ctx, suite.iterationId)

	suite.NoError(err)
	suite.Assert().Equal(1, len(toCreate))
	suite.Assert().Equal(1, numPending)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_GetIndexesToCreate_security_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(suite.scenarioAndIteration, nil)
	suite.enforceSecurity.On("PublishScenario", suite.scenario).Return(suite.securityError)

	_, _, err := suite.makeUsecase().GetIndexesToCreate(suite.ctx, suite.iterationId)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_GetIndexesToCreate_fetch_error() {
	suite.executorFactory.On("NewExecutor").Return(suite.transaction)
	suite.scenarioFetcher.On("FetchScenarioAndIteration", suite.ctx, suite.transaction,
		suite.iterationId).Return(models.ScenarioAndIteration{}, suite.repositoryError)

	_, _, err := suite.makeUsecase().GetIndexesToCreate(suite.ctx, suite.iterationId)

	suite.Assert().ErrorIs(err, suite.repositoryError)

	suite.AssertExpectations()
}

// CreateIndexesAsync
func (suite *ClientDbIndexEditorTestSuite) Test_CreateIndexesAsync_nominal() {
	indexes := []models.ConcreteIndex{
		{
			TableName: "table", Indexed: []string{"a", "b"},
			Included: []string{"c", "d"},
		},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("CreateIndexesAsync", suite.ctx, suite.transaction, indexes).Return(nil)

	err := suite.makeUsecase().CreateIndexesAsync(suite.ctx, indexes)

	suite.NoError(err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_CreateIndexesAsync_security_error() {
	indexes := []models.ConcreteIndex{
		{
			TableName: "table", Indexed: []string{"a", "b"},
			Included: []string{"c", "d"},
		},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := suite.makeUsecase().CreateIndexesAsync(suite.ctx, indexes)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_CreateIndexesAsync_error() {
	indexes := []models.ConcreteIndex{
		{
			TableName: "table", Indexed: []string{"a", "b"},
			Included: []string{"c", "d"},
		},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("CreateIndexesAsync", suite.ctx, suite.transaction, indexes).Return(suite.repositoryError)

	err := suite.makeUsecase().CreateIndexesAsync(suite.ctx, indexes)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_CreateIndexesAsync_get_executor_error() {
	indexes := []models.ConcreteIndex{
		{
			TableName: "table", Indexed: []string{"a", "b"},
			Included: []string{"c", "d"},
		},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(nil, suite.repositoryError)

	err := suite.makeUsecase().CreateIndexesAsync(suite.ctx, indexes)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

// ListAllUniqueIndexes
func (suite *ClientDbIndexEditorTestSuite) Test_ListAllUniqueIndexes_nominal() {
	uniqueIndexes := []models.UnicityIndex{
		{
			TableName: "table", Fields: []string{"a", "b"},
		},
	}
	suite.enforceSecurityDataModel.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("ListAllUniqueIndexes", suite.ctx, suite.transaction).Return(uniqueIndexes, nil)

	indexes, err := suite.makeUsecase().ListAllUniqueIndexes(suite.ctx)

	suite.NoError(err)
	suite.Assert().Equal(uniqueIndexes, indexes)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_ListAllUniqueIndexes_security_error() {
	suite.enforceSecurityDataModel.On("ReadDataModel").Return(suite.securityError)

	_, err := suite.makeUsecase().ListAllUniqueIndexes(suite.ctx)

	suite.Assert().Error(err)
	suite.Assert().Equal(suite.securityError, err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_ListAllUniqueIndexes_repository_error() {
	suite.enforceSecurityDataModel.On("ReadDataModel").Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("ListAllUniqueIndexes", suite.ctx, suite.transaction).Return(nil, suite.repositoryError)

	_, err := suite.makeUsecase().ListAllUniqueIndexes(suite.ctx)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

// CreateUniqueIndexAsync
func (suite *ClientDbIndexEditorTestSuite) Test_CreateUniqueIndexAsync_nominal() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("CreateUniqueIndexAsync", suite.ctx, suite.transaction, index).Return(nil)

	err := suite.makeUsecase().CreateUniqueIndexAsync(suite.ctx, index)

	suite.NoError(err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_CreateUniqueIndexAsync_security_error() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := suite.makeUsecase().CreateUniqueIndexAsync(suite.ctx, index)

	suite.Assert().Error(err)
	suite.Assert().Equal(suite.securityError, err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_CreateUniqueIndexAsync_error() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(nil, suite.repositoryError)

	err := suite.makeUsecase().CreateUniqueIndexAsync(suite.ctx, index)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

// CreateUniqueIndex
func (suite *ClientDbIndexEditorTestSuite) Test_CreateUniqueIndex_nominal() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("CreateUniqueIndex", suite.ctx, suite.transaction, index).Return(nil)

	err := suite.makeUsecase().CreateUniqueIndex(suite.ctx, nil, index)

	suite.NoError(err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_CreateUniqueIndex_security_error() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := suite.makeUsecase().CreateUniqueIndex(suite.ctx, nil, index)

	suite.Assert().Error(err)
	suite.Assert().Equal(suite.securityError, err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_CreateUniqueIndex_error() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("CreateUniqueIndex", suite.ctx, suite.transaction, index).Return(suite.repositoryError)

	err := suite.makeUsecase().CreateUniqueIndex(suite.ctx, nil, index)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

// DeleteUniqueIndex
func (suite *ClientDbIndexEditorTestSuite) Test_DeleteUniqueIndex_nominal() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("DeleteUniqueIndex", suite.ctx, suite.transaction, index).Return(nil)

	err := suite.makeUsecase().DeleteUniqueIndex(suite.ctx, index)

	suite.NoError(err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_DeleteUniqueIndex_security_error() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(suite.securityError)

	err := suite.makeUsecase().DeleteUniqueIndex(suite.ctx, index)

	suite.Assert().Error(err)
	suite.Assert().Equal(suite.securityError, err)

	suite.AssertExpectations()
}

func (suite *ClientDbIndexEditorTestSuite) Test_DeleteUniqueIndex_error() {
	index := models.UnicityIndex{
		TableName: "table", Fields: []string{"a", "b"},
	}
	suite.enforceSecurityDataModel.On("WriteDataModel", suite.organizationId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.organizationId).Return(suite.transaction, nil)
	suite.ingestedDataIndexesRepository.On("DeleteUniqueIndex", suite.ctx, suite.transaction, index).Return(suite.repositoryError)

	err := suite.makeUsecase().DeleteUniqueIndex(suite.ctx, index)

	suite.Assert().Error(err)

	suite.AssertExpectations()
}

func TestClientDbIndexEditor(t *testing.T) {
	suite.Run(t, new(ClientDbIndexEditorTestSuite))
}
