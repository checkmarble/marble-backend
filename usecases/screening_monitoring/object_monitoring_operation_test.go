package screening_monitoring

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ScreeningMonitoringUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity              *mocks.EnforceSecurity
	repository                   *mocks.ScreeningMonitoringRepository
	clientDbRepository           *mocks.ScreeningMonitoringClientDbRepository
	organizationSchemaRepository *mocks.OrganizationSchemaRepository
	ingestedDataReader           *mocks.ScreeningMonitoringIngestedDataReader
	ingestionUsecase             *mocks.ScreeningMonitoringIngestionUsecase
	executorFactory              executor_factory.ExecutorFactoryStub
	transactionFactory           executor_factory.TransactionFactoryStub

	ctx        context.Context
	configId   uuid.UUID
	orgId      string
	objectType string
	objectId   string
}

func (suite *ScreeningMonitoringUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.repository = new(mocks.ScreeningMonitoringRepository)
	suite.clientDbRepository = new(mocks.ScreeningMonitoringClientDbRepository)
	suite.organizationSchemaRepository = new(mocks.OrganizationSchemaRepository)
	suite.ingestedDataReader = new(mocks.ScreeningMonitoringIngestedDataReader)
	suite.ingestionUsecase = new(mocks.ScreeningMonitoringIngestionUsecase)

	suite.executorFactory = executor_factory.NewExecutorFactoryStub()
	suite.transactionFactory = executor_factory.NewTransactionFactoryStub(suite.executorFactory)

	suite.ctx = context.Background()
	suite.configId = uuid.New()
	suite.orgId = "test-org-id"
	suite.objectType = "transactions"
	suite.objectId = "test-object-id"
}

func (suite *ScreeningMonitoringUsecaseTestSuite) makeUsecase() *ScreeningMonitoringUsecase {
	return &ScreeningMonitoringUsecase{
		executorFactory:              suite.executorFactory,
		transactionFactory:           suite.transactionFactory,
		enforceSecurity:              suite.enforceSecurity,
		repository:                   suite.repository,
		clientDbRepository:           suite.clientDbRepository,
		organizationSchemaRepository: suite.organizationSchemaRepository,
		ingestedDataReader:           suite.ingestedDataReader,
		ingestionUsecase:             suite.ingestionUsecase,
	}
}

func (suite *ScreeningMonitoringUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.repository.AssertExpectations(t)
	suite.clientDbRepository.AssertExpectations(t)
	suite.organizationSchemaRepository.AssertExpectations(t)
	suite.ingestedDataReader.AssertExpectations(t)
	suite.ingestionUsecase.AssertExpectations(t)
}

func TestScreeningMonitoringUsecase(t *testing.T) {
	suite.Run(t, new(ScreeningMonitoringUsecaseTestSuite))
}

func (suite *ScreeningMonitoringUsecaseTestSuite) TestInsertScreeningMonitoringObject_WithObjectId() {
	// Setup test data
	config := models.ScreeningMonitoringConfig{
		Id:    suite.configId,
		OrgId: suite.orgId,
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	table := models.Table{
		Name:      suite.objectType,
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
			suite.objectType: table,
		},
	}

	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
		},
	}

	// Setup expectations
	suite.repository.On("GetScreeningMonitoringConfig", suite.ctx, mock.Anything, suite.configId).Return(config, nil)
	suite.enforceSecurity.On("WriteMonitoredObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, mock.Anything, table,
		suite.objectId).Return(ingestedObjects, nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil)
	suite.clientDbRepository.On("CreateInternalScreeningMonitoringTable", suite.ctx,
		mock.Anything, suite.objectType).Return(nil)
	suite.clientDbRepository.On("InsertScreeningMonitoringObject", suite.ctx, mock.Anything,
		suite.objectType, suite.objectId, suite.configId).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertScreeningMonitoringObject{
		ObjectType: suite.objectType,
		ConfigId:   suite.configId,
		ObjectId:   &suite.objectId,
	}

	err := uc.InsertScreeningMonitoringObject(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScreeningMonitoringUsecaseTestSuite) TestInsertScreeningMonitoringObject_WithObjectPayload() {
	payload := json.RawMessage(`{"object_id": "test-object-id", "amount": 100}`)

	// Setup test data
	config := models.ScreeningMonitoringConfig{
		Id:    suite.configId,
		OrgId: suite.orgId,
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	table := models.Table{
		Name:      suite.objectType,
		FTMEntity: &ftmEntityValue,
		Fields: map[string]models.Field{
			"object_id": {
				Name:        "object_id",
				FTMProperty: &ftmPropertyValue,
			},
			"amount": {
				Name:     "amount",
				DataType: models.Float,
			},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			suite.objectType: table,
		},
	}

	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
		},
	}

	// Setup expectations
	suite.repository.On("GetScreeningMonitoringConfig", suite.ctx, mock.Anything, suite.configId).Return(config, nil)
	suite.enforceSecurity.On("WriteMonitoredObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.ingestionUsecase.On("IngestObject", suite.ctx, suite.orgId, suite.objectType, payload).Return(1, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, mock.Anything, table,
		suite.objectId).Return(ingestedObjects, nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil)
	suite.clientDbRepository.On("CreateInternalScreeningMonitoringTable", suite.ctx,
		mock.Anything, suite.objectType).Return(nil)
	suite.clientDbRepository.On("InsertScreeningMonitoringObject", suite.ctx, mock.Anything,
		suite.objectType, suite.objectId, suite.configId).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertScreeningMonitoringObject{
		ObjectType:    suite.objectType,
		ConfigId:      suite.configId,
		ObjectPayload: &payload,
	}

	err := uc.InsertScreeningMonitoringObject(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScreeningMonitoringUsecaseTestSuite) TestInsertScreeningMonitoringObject_TableNotConfigured() {
	// Setup test data - table without FTM entity
	config := models.ScreeningMonitoringConfig{
		Id:    suite.configId,
		OrgId: suite.orgId,
	}

	table := models.Table{
		Name:      suite.objectType,
		FTMEntity: nil, // Missing FTM entity
		Fields: map[string]models.Field{
			"object_id": {
				Name: "object_id",
			},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			suite.objectType: table,
		},
	}

	// Setup expectations
	suite.repository.On("GetScreeningMonitoringConfig", suite.ctx, mock.Anything, suite.configId).Return(config, nil)
	suite.enforceSecurity.On("WriteMonitoredObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertScreeningMonitoringObject{
		ObjectType: suite.objectType,
		ConfigId:   suite.configId,
		ObjectId:   &suite.objectId,
	}

	err := uc.InsertScreeningMonitoringObject(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "table is not configured for the use case")
	suite.AssertExpectations()
}

func (suite *ScreeningMonitoringUsecaseTestSuite) TestInsertScreeningMonitoringObject_ObjectIdNotFoundInIngestedData() {
	// Setup test data
	config := models.ScreeningMonitoringConfig{
		Id:    suite.configId,
		OrgId: suite.orgId,
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	table := models.Table{
		Name:      suite.objectType,
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
			suite.objectType: table,
		},
	}

	// Setup expectations - QueryIngestedObject returns empty list
	suite.repository.On("GetScreeningMonitoringConfig", suite.ctx, mock.Anything, suite.configId).Return(config, nil)
	suite.enforceSecurity.On("WriteMonitoredObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, mock.Anything, table,
		suite.objectId).Return([]models.DataModelObject{}, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertScreeningMonitoringObject{
		ObjectType: suite.objectType,
		ConfigId:   suite.configId,
		ObjectId:   &suite.objectId,
	}

	err := uc.InsertScreeningMonitoringObject(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "object test-object-id not found in ingested data")
	suite.AssertExpectations()
}

func (suite *ScreeningMonitoringUsecaseTestSuite) TestInsertScreeningMonitoringObject_ObjectPayloadNotIngested() {
	// Setup test data - payload with object_id
	payload := json.RawMessage(`{"object_id": "test-object-id", "amount": 100}`)

	config := models.ScreeningMonitoringConfig{
		Id:    suite.configId,
		OrgId: suite.orgId,
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	table := models.Table{
		Name:      suite.objectType,
		FTMEntity: &ftmEntityValue,
		Fields: map[string]models.Field{
			"object_id": {
				Name:        "object_id",
				FTMProperty: &ftmPropertyValue,
			},
			"amount": {
				Name:     "amount",
				DataType: models.Float,
			},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			suite.objectType: table,
		},
	}

	// Setup expectations - IngestObject returns 0 (no objects ingested)
	suite.repository.On("GetScreeningMonitoringConfig", suite.ctx, mock.Anything, suite.configId).Return(config, nil)
	suite.enforceSecurity.On("WriteMonitoredObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.ingestionUsecase.On("IngestObject", suite.ctx, suite.orgId, suite.objectType, payload).Return(0, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertScreeningMonitoringObject{
		ObjectType:    suite.objectType,
		ConfigId:      suite.configId,
		ObjectPayload: &payload,
	}

	err := uc.InsertScreeningMonitoringObject(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "no object ingested")
	suite.AssertExpectations()
}

func (suite *ScreeningMonitoringUsecaseTestSuite) TestInsertScreeningMonitoringObject_UniqueViolationWithIgnoreConflictError() {
	// Setup test data - object payload, which will set ignoreConflictError to true
	payload := json.RawMessage(`{"object_id": "test-object-id", "amount": 100}`)

	config := models.ScreeningMonitoringConfig{
		Id:    suite.configId,
		OrgId: suite.orgId,
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	table := models.Table{
		Name:      suite.objectType,
		FTMEntity: &ftmEntityValue,
		Fields: map[string]models.Field{
			"object_id": {
				Name:        "object_id",
				FTMProperty: &ftmPropertyValue,
			},
			"amount": {
				Name:     "amount",
				DataType: models.Float,
			},
		},
	}

	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			suite.objectType: table,
		},
	}

	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
		},
	}

	// Setup expectations
	suite.repository.On("GetScreeningMonitoringConfig", suite.ctx, mock.Anything, suite.configId).Return(config, nil)
	suite.enforceSecurity.On("WriteMonitoredObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.ingestionUsecase.On("IngestObject", suite.ctx, suite.orgId, suite.objectType, payload).Return(1, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, mock.Anything, table,
		suite.objectId).Return(ingestedObjects, nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil)
	suite.clientDbRepository.On("CreateInternalScreeningMonitoringTable", suite.ctx,
		mock.Anything, suite.objectType).Return(nil)
	// Return a unique violation error
	suite.clientDbRepository.On("InsertScreeningMonitoringObject", suite.ctx, mock.Anything,
		suite.objectType, suite.objectId, suite.configId).Return(&pgconn.PgError{
		Code: pgerrcode.UniqueViolation,
	})

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertScreeningMonitoringObject{
		ObjectType:    suite.objectType,
		ConfigId:      suite.configId,
		ObjectPayload: &payload,
	}

	err := uc.InsertScreeningMonitoringObject(suite.ctx, input)

	// Assert - should not error when ignoreConflictError is true and unique violation occurs
	suite.NoError(err)
	suite.AssertExpectations()
}

func (suite *ScreeningMonitoringUsecaseTestSuite) TestInsertScreeningMonitoringObject_UniqueViolationWithoutIgnoreConflictError() {
	// Setup test data - object ID, which will NOT set ignoreConflictError
	config := models.ScreeningMonitoringConfig{
		Id:    suite.configId,
		OrgId: suite.orgId,
	}

	ftmEntityValue := models.FollowTheMoneyEntityPerson
	ftmPropertyValue := models.FollowTheMoneyPropertyName
	table := models.Table{
		Name:      suite.objectType,
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
			suite.objectType: table,
		},
	}

	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
		},
	}

	// Setup expectations
	suite.repository.On("GetScreeningMonitoringConfig", suite.ctx, mock.Anything, suite.configId).Return(config, nil)
	suite.enforceSecurity.On("WriteMonitoredObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", suite.ctx, mock.Anything, suite.orgId, false, false).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, mock.Anything, table,
		suite.objectId).Return(ingestedObjects, nil)
	suite.organizationSchemaRepository.On("CreateSchemaIfNotExists", suite.ctx, mock.Anything).Return(nil)
	suite.clientDbRepository.On("CreateInternalScreeningMonitoringTable", suite.ctx,
		mock.Anything, suite.objectType).Return(nil)
	// Return a unique violation error
	suite.clientDbRepository.On("InsertScreeningMonitoringObject", suite.ctx, mock.Anything,
		suite.objectType, suite.objectId, suite.configId).Return(&pgconn.PgError{
		Code: pgerrcode.UniqueViolation,
	})

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertScreeningMonitoringObject{
		ObjectType: suite.objectType,
		ConfigId:   suite.configId,
		ObjectId:   &suite.objectId,
	}

	err := uc.InsertScreeningMonitoringObject(suite.ctx, input)

	// Assert - should error when ignoreConflictError is false and unique violation occurs
	suite.Error(err)
	suite.Contains(err.Error(), "object already exists in screening monitored objects table")
	suite.AssertExpectations()
}

func TestExtractObjectIDFromPayload(t *testing.T) {
	tests := []struct {
		name      string
		payload   json.RawMessage
		expected  string
		wantError bool
	}{
		{
			name:      "valid payload",
			payload:   json.RawMessage(`{"object_id": "test-123"}`),
			expected:  "test-123",
			wantError: false,
		},
		{
			name:      "payload with extra fields",
			payload:   json.RawMessage(`{"object_id": "test-456", "amount": 100, "currency": "USD"}`),
			expected:  "test-456",
			wantError: false,
		},
		{
			name:      "invalid json",
			payload:   json.RawMessage(`{invalid json}`),
			expected:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractObjectIDFromPayload(tt.payload)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestCheckDataModelTableAndFieldsConfiguration(t *testing.T) {
	ftmEntity := models.FollowTheMoneyEntityPerson
	ftmProperty := models.FollowTheMoneyPropertyName

	tests := []struct {
		name      string
		table     models.Table
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid configuration",
			table: models.Table{
				Name:      "transactions",
				FTMEntity: &ftmEntity,
				Fields: map[string]models.Field{
					"object_id": {
						Name:        "object_id",
						FTMProperty: &ftmProperty,
					},
				},
			},
			wantError: false,
		},
		{
			name: "missing FTM entity",
			table: models.Table{
				Name:      "transactions",
				FTMEntity: nil,
				Fields: map[string]models.Field{
					"object_id": {
						Name:        "object_id",
						FTMProperty: &ftmProperty,
					},
				},
			},
			wantError: true,
			errorMsg:  "table is not configured for the use case",
		},
		{
			name: "missing FTM property on fields",
			table: models.Table{
				Name:      "transactions",
				FTMEntity: &ftmEntity,
				Fields: map[string]models.Field{
					"object_id": {
						Name:        "object_id",
						FTMProperty: nil,
					},
				},
			},
			wantError: true,
			errorMsg:  "table's fields are not configured for the use case",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDataModelTableAndFieldsConfiguration(tt.table)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
