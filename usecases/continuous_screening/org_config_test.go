package continuous_screening

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type OrgConfigTestSuite struct {
	suite.Suite
	enforceSecurity              *mocks.EnforceSecurity
	repository                   *mocks.ContinuousScreeningRepository
	clientDbRepository           *mocks.ContinuousScreeningClientDbRepository
	organizationSchemaRepository *mocks.OrganizationSchemaRepository
	screeningProvider            *mocks.ContinuousScreeningScreeningProvider
	executorFactory              executor_factory.ExecutorFactoryStub
	transactionFactory           executor_factory.TransactionFactoryStub

	ctx      context.Context
	orgId    string
	configId uuid.UUID
}

func (suite *OrgConfigTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.clientDbRepository = new(mocks.ContinuousScreeningClientDbRepository)
	suite.organizationSchemaRepository = new(mocks.OrganizationSchemaRepository)
	suite.screeningProvider = new(mocks.ContinuousScreeningScreeningProvider)

	suite.executorFactory = executor_factory.NewExecutorFactoryStub()
	suite.transactionFactory = executor_factory.NewTransactionFactoryStub(suite.executorFactory)

	suite.ctx = context.Background()
	suite.orgId = "test-org-id"
	suite.configId = uuid.New()
}

func (suite *OrgConfigTestSuite) makeUsecase() *ContinuousScreeningUsecase {
	return &ContinuousScreeningUsecase{
		executorFactory:              suite.executorFactory,
		transactionFactory:           suite.transactionFactory,
		enforceSecurity:              suite.enforceSecurity,
		repository:                   suite.repository,
		clientDbRepository:           suite.clientDbRepository,
		organizationSchemaRepository: suite.organizationSchemaRepository,
		screeningProvider:            suite.screeningProvider,
	}
}

func (suite *OrgConfigTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.repository.AssertExpectations(t)
	suite.clientDbRepository.AssertExpectations(t)
	suite.organizationSchemaRepository.AssertExpectations(t)
	suite.screeningProvider.AssertExpectations(t)
}

func TestOrgConfigTestSuite(t *testing.T) {
	suite.Run(t, new(OrgConfigTestSuite))
}

func (suite *OrgConfigTestSuite) TestCreateContinuousScreeningConfig_InvalidAlgorithm() {
	// Setup
	input := models.CreateContinuousScreeningConfig{
		OrgId:       suite.orgId,
		Algorithm:   "invalid-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(models.OpenSanctionAlgorithms{}, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.CreateContinuousScreeningConfig(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "bad parameter")
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestCreateContinuousScreeningConfig() {
	// Setup
	input := models.CreateContinuousScreeningConfig{
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	table := models.Table{
		Name:      "transactions",
		FTMEntity: &ftmEntityValue,
		Fields: map[string]models.Field{
			"object_id": {
				Name:        "object_id",
				FTMProperty: &ftmPropertyValue,
			},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"transactions": table,
		},
	}

	expectedConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	algorithms := models.OpenSanctionAlgorithms{
		Algorithms: []models.OpenSanctionAlgorithm{
			{Name: "valid-algorithm"},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(algorithms, nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil)
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx,
		mock.Anything, "transactions").Return(nil)
	suite.repository.On("CreateContinuousScreeningConfig", suite.ctx, mock.Anything, input).Return(expectedConfig, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.CreateContinuousScreeningConfig(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(expectedConfig, result)
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestCreateContinuousScreeningConfig_EmptyObjectTypes() {
	// Setup
	input := models.CreateContinuousScreeningConfig{
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{}, // Empty object types
	}

	algorithms := models.OpenSanctionAlgorithms{
		Algorithms: []models.OpenSanctionAlgorithm{
			{Name: "valid-algorithm"},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(algorithms, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.CreateContinuousScreeningConfig(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "object_types cannot be empty")
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestCreateContinuousScreeningConfig_NonEmptyObjectTypes() {
	// Setup
	input := models.CreateContinuousScreeningConfig{
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{"transactions", "customers"},
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	tables := map[string]models.Table{
		"transactions": {
			Name:      "transactions",
			FTMEntity: &ftmEntityValue,
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
		"customers": {
			Name:      "customers",
			FTMEntity: &ftmEntityValue,
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
	}

	dataModel := models.DataModel{
		Tables: tables,
	}

	expectedConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{"transactions", "customers"},
	}

	algorithms := models.OpenSanctionAlgorithms{
		Algorithms: []models.OpenSanctionAlgorithm{
			{Name: "valid-algorithm"},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(algorithms, nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil)
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx,
		mock.Anything, "transactions").Return(nil)
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx, mock.Anything, "customers").Return(nil)
	suite.repository.On("CreateContinuousScreeningConfig", suite.ctx, mock.Anything, input).Return(expectedConfig, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.CreateContinuousScreeningConfig(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(expectedConfig, result)
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestUpdateContinuousScreeningConfig_InvalidAlgorithm() {
	// Setup
	invalidAlgorithm := "invalid-algorithm"
	input := models.UpdateContinuousScreeningConfig{
		Algorithm: &invalidAlgorithm,
	}

	existingConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "existing-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningConfig", suite.ctx, mock.Anything,
		suite.configId).Return(existingConfig, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(models.OpenSanctionAlgorithms{}, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpdateContinuousScreeningConfig(suite.ctx, suite.configId, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "bad parameter")
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestUpdateContinuousScreeningConfig_ValidAlgorithm() {
	// Setup
	validAlgorithm := "valid-algorithm"
	input := models.UpdateContinuousScreeningConfig{
		Algorithm: &validAlgorithm,
	}

	existingConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "existing-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	updatedConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	algorithms := models.OpenSanctionAlgorithms{
		Algorithms: []models.OpenSanctionAlgorithm{
			{Name: "valid-algorithm"},
		},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningConfig", suite.ctx, mock.Anything,
		suite.configId).Return(existingConfig, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(algorithms, nil)
	suite.repository.On("UpdateContinuousScreeningConfig", suite.ctx, mock.Anything,
		suite.configId, input).Return(updatedConfig, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningConfig(suite.ctx, suite.configId, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedConfig, result)
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestUpdateContinuousScreeningConfig_RemoveObjectTypes() {
	// Setup - trying to remove object types (should fail)
	newObjectTypes := []string{"transactions"} // Removing "customers"
	input := models.UpdateContinuousScreeningConfig{
		ObjectTypes: &newObjectTypes,
	}

	existingConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "existing-algorithm",
		ObjectTypes: []string{"transactions", "customers"}, // Original has both
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningConfig", suite.ctx, mock.Anything,
		suite.configId).Return(existingConfig, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpdateContinuousScreeningConfig(suite.ctx, suite.configId, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "cannot remove object types")
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestUpdateContinuousScreeningConfig_AddObjectTypes() {
	// Setup - adding new object types (should succeed)
	newObjectTypes := []string{"transactions", "customers", "accounts"} // Adding "accounts"
	input := models.UpdateContinuousScreeningConfig{
		ObjectTypes: &newObjectTypes,
	}

	existingConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "existing-algorithm",
		ObjectTypes: []string{"transactions", "customers"}, // Original has both
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	tables := map[string]models.Table{
		"transactions": {
			Name:      "transactions",
			FTMEntity: &ftmEntityValue,
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
		"customers": {
			Name:      "customers",
			FTMEntity: &ftmEntityValue,
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
		"accounts": {
			Name:      "accounts",
			FTMEntity: &ftmEntityValue,
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
	}

	dataModel := models.DataModel{
		Tables: tables,
	}

	updatedConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "existing-algorithm",
		ObjectTypes: []string{"transactions", "customers", "accounts"},
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningConfig", suite.ctx, mock.Anything,
		suite.configId).Return(existingConfig, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil)
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx,
		mock.Anything, "transactions").Return(nil)
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx, mock.Anything, "customers").Return(nil)
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx, mock.Anything, "accounts").Return(nil)
	suite.repository.On("UpdateContinuousScreeningConfig", suite.ctx, mock.Anything,
		suite.configId, input).Return(updatedConfig, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpdateContinuousScreeningConfig(suite.ctx, suite.configId, input)

	// Assert
	suite.NoError(err)
	suite.Equal(updatedConfig, result)
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestCreateContinuousScreeningConfig_TableMissingFTMEntity() {
	// Setup - table without FTM entity
	input := models.CreateContinuousScreeningConfig{
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	ftmPropertyValue := models.FollowTheMoneyPropertyName
	table := models.Table{
		Name:      "transactions",
		FTMEntity: nil, // Missing FTM entity
		Fields: map[string]models.Field{
			"object_id": {
				Name:        "object_id",
				FTMProperty: &ftmPropertyValue,
			},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"transactions": table,
		},
	}

	algorithms := models.OpenSanctionAlgorithms{
		Algorithms: []models.OpenSanctionAlgorithm{
			{Name: "valid-algorithm"},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(algorithms, nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.CreateContinuousScreeningConfig(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "table is not configured for the use case")
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestCreateContinuousScreeningConfig_TableMissingFTMProperty() {
	// Setup - table with field missing FTM property
	input := models.CreateContinuousScreeningConfig{
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	table := models.Table{
		Name:      "transactions",
		FTMEntity: &ftmEntityValue,
		Fields: map[string]models.Field{
			"object_id": {
				Name:        "object_id",
				FTMProperty: nil, // Missing FTM property
			},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"transactions": table,
		},
	}

	algorithms := models.OpenSanctionAlgorithms{
		Algorithms: []models.OpenSanctionAlgorithm{
			{Name: "valid-algorithm"},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(algorithms, nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.CreateContinuousScreeningConfig(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "table's fields are not configured for the use case")
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestCreateContinuousScreeningConfig_ObjectTypeNotFound() {
	// Setup - object type doesn't exist in data model
	input := models.CreateContinuousScreeningConfig{
		OrgId:       suite.orgId,
		Algorithm:   "valid-algorithm",
		ObjectTypes: []string{"nonexistent_table"},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			// Empty tables map - the nonexistent_table won't be found
		},
	}

	algorithms := models.OpenSanctionAlgorithms{
		Algorithms: []models.OpenSanctionAlgorithm{
			{Name: "valid-algorithm"},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.screeningProvider.On("GetAlgorithms", suite.ctx).Return(algorithms, nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.CreateContinuousScreeningConfig(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "table nonexistent_table not found in data model")
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestUpdateContinuousScreeningConfig_AddNonExistentObjectType() {
	// Setup - adding an object type that doesn't exist in data model
	newObjectTypes := []string{"transactions", "nonexistent_table"} // nonexistent_table doesn't exist
	input := models.UpdateContinuousScreeningConfig{
		ObjectTypes: &newObjectTypes,
	}

	existingConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "existing-algorithm",
		ObjectTypes: []string{"transactions"},
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	tables := map[string]models.Table{
		"transactions": {
			Name:      "transactions",
			FTMEntity: &ftmEntityValue,
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
		// Note: nonexistent_table is not in the tables map
	}

	dataModel := models.DataModel{
		Tables: tables,
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningConfig", suite.ctx, mock.Anything,
		suite.configId).Return(existingConfig, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	// The valid table "transactions" will be processed first before failing on the nonexistent table
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil)
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx,
		mock.Anything, "transactions").Return(nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpdateContinuousScreeningConfig(suite.ctx, suite.configId, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "table nonexistent_table not found in data model")
	suite.AssertExpectations()
}

func (suite *OrgConfigTestSuite) TestUpdateContinuousScreeningConfig_AddInvalidTable() {
	// Setup - adding a table that has missing FTM entity
	newObjectTypes := []string{"transactions", "customers", "invalid_table"} // Adding "invalid_table" which has no FTM entity
	input := models.UpdateContinuousScreeningConfig{
		ObjectTypes: &newObjectTypes,
	}

	existingConfig := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		OrgId:       suite.orgId,
		Algorithm:   "existing-algorithm",
		ObjectTypes: []string{"transactions", "customers"},
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	tables := map[string]models.Table{
		"transactions": {
			Name:      "transactions",
			FTMEntity: &ftmEntityValue,
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
		"customers": {
			Name:      "customers",
			FTMEntity: &ftmEntityValue,
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
		"invalid_table": {
			Name:      "invalid_table",
			FTMEntity: nil, // Missing FTM entity
			Fields: map[string]models.Field{
				"object_id": {
					Name:        "object_id",
					FTMProperty: &ftmPropertyValue,
				},
			},
		},
	}

	dataModel := models.DataModel{
		Tables: tables,
	}

	// Mock expectations
	suite.repository.On("GetContinuousScreeningConfig", suite.ctx, mock.Anything,
		suite.configId).Return(existingConfig, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningConfig", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	// Schema and table creation will be called for the first two valid tables before validation fails on the third
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil).Twice()
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx,
		mock.Anything, "transactions").Return(nil)
	suite.clientDbRepository.On("CreateInternalContinuousScreeningTable", suite.ctx,
		mock.Anything, "customers").Return(nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpdateContinuousScreeningConfig(suite.ctx, suite.configId, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "table is not configured for the use case")
	suite.AssertExpectations()
}
