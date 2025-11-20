package continuous_screening

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

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

type ContinuousScreeningUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity              *mocks.EnforceSecurity
	repository                   *mocks.ContinuousScreeningRepository
	clientDbRepository           *mocks.ContinuousScreeningClientDbRepository
	organizationSchemaRepository *mocks.OrganizationSchemaRepository
	ingestedDataReader           *mocks.ContinuousScreeningIngestedDataReader
	ingestionUsecase             *mocks.ContinuousScreeningIngestionUsecase
	screeningProvider            *mocks.ContinuousScreeningScreeningProvider
	caseEditor                   *mocks.CaseEditor
	executorFactory              executor_factory.ExecutorFactoryStub
	transactionFactory           executor_factory.TransactionFactoryStub

	ctx            context.Context
	configId       uuid.UUID
	configStableId uuid.UUID
	orgId          uuid.UUID
	objectType     string
	objectId       string
	caseId         uuid.UUID
}

func (suite *ContinuousScreeningUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.repository = new(mocks.ContinuousScreeningRepository)
	suite.clientDbRepository = new(mocks.ContinuousScreeningClientDbRepository)
	suite.organizationSchemaRepository = new(mocks.OrganizationSchemaRepository)
	suite.ingestedDataReader = new(mocks.ContinuousScreeningIngestedDataReader)
	suite.ingestionUsecase = new(mocks.ContinuousScreeningIngestionUsecase)
	suite.screeningProvider = new(mocks.ContinuousScreeningScreeningProvider)
	suite.caseEditor = new(mocks.CaseEditor)

	suite.executorFactory = executor_factory.NewExecutorFactoryStub()
	suite.transactionFactory = executor_factory.NewTransactionFactoryStub(suite.executorFactory)

	suite.ctx = context.Background()
	suite.configId = uuid.New()
	suite.configStableId = uuid.New()
	suite.orgId = uuid.MustParse("12345678-1234-1234-1234-123456789012")
	suite.objectType = "transactions"
	suite.objectId = "test-object-id"
	suite.caseId = uuid.New()
}

func (suite *ContinuousScreeningUsecaseTestSuite) makeUsecase() *ContinuousScreeningUsecase {
	return &ContinuousScreeningUsecase{
		executorFactory:              suite.executorFactory,
		transactionFactory:           suite.transactionFactory,
		enforceSecurity:              suite.enforceSecurity,
		repository:                   suite.repository,
		clientDbRepository:           suite.clientDbRepository,
		organizationSchemaRepository: suite.organizationSchemaRepository,
		ingestedDataReader:           suite.ingestedDataReader,
		ingestionUsecase:             suite.ingestionUsecase,
		screeningProvider:            suite.screeningProvider,
		caseEditor:                   suite.caseEditor,
	}
}

func (suite *ContinuousScreeningUsecaseTestSuite) AssertExpectations() {
	t := suite.T()
	suite.enforceSecurity.AssertExpectations(t)
	suite.repository.AssertExpectations(t)
	suite.clientDbRepository.AssertExpectations(t)
	suite.organizationSchemaRepository.AssertExpectations(t)
	suite.ingestedDataReader.AssertExpectations(t)
	suite.ingestionUsecase.AssertExpectations(t)
	suite.screeningProvider.AssertExpectations(t)
	suite.caseEditor.AssertExpectations(t)
}

func TestContinuousScreeningUsecase(t *testing.T) {
	suite.Run(t, new(ContinuousScreeningUsecaseTestSuite))
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_WithObjectId() {
	// Setup test data
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
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

	objectInternalId := uuid.New()
	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
			Metadata: map[string]any{
				"id": [16]byte(objectInternalId),
			},
		},
	}

	// Setup expectations
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table,
		suite.objectId, mock.Anything).Return(ingestedObjects, nil)
	suite.screeningProvider.On("Search", mock.Anything, mock.MatchedBy(func(query models.OpenSanctionsQuery) bool {
		return len(query.Queries) > 0
	})).Return(models.ScreeningRawSearchResponseWithMatches{
		SearchInput:       []byte("{}"),
		InitialHasMatches: false,
		Matches:           []models.ScreeningMatch{},
	}, nil)
	suite.clientDbRepository.On("InsertContinuousScreeningObject", mock.Anything, mock.Anything,
		suite.objectType, suite.objectId, suite.configStableId).Return(nil)
	suite.repository.On("InsertContinuousScreening", mock.Anything, mock.Anything, mock.Anything,
		config, suite.objectType,
		suite.objectId, mock.Anything, mock.Anything).Return(models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                                uuid.New(),
			OrgId:                             uuid.New(),
			ContinuousScreeningConfigId:       suite.configId,
			ContinuousScreeningConfigStableId: suite.configStableId,
			ObjectType:                        suite.objectType,
			ObjectId:                          suite.objectId,
		},
		Matches: []models.ContinuousScreeningMatch{},
	}, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectId:       &suite.objectId,
	}

	result, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.NotNil(result)
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_WithObjectPayload() {
	payload := json.RawMessage(`{"object_id": "test-object-id", "amount": 100}`)

	// Setup test data
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
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

	objectInternalId := uuid.New()
	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
			Metadata: map[string]any{
				"id": [16]byte(objectInternalId),
			},
		},
	}

	// Setup expectations
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)
	suite.ingestionUsecase.On("IngestObject", mock.Anything, suite.orgId.String(),
		suite.objectType, payload).Return(1, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table,
		suite.objectId, mock.Anything).Return(ingestedObjects, nil)
	suite.screeningProvider.On("Search", mock.Anything, mock.MatchedBy(func(query models.OpenSanctionsQuery) bool {
		return len(query.Queries) > 0
	})).Return(models.ScreeningRawSearchResponseWithMatches{
		SearchInput:       []byte("{}"),
		InitialHasMatches: false,
		Matches:           []models.ScreeningMatch{},
	}, nil)
	suite.clientDbRepository.On("InsertContinuousScreeningObject", mock.Anything, mock.Anything,
		suite.objectType, suite.objectId, suite.configStableId).Return(nil)
	suite.repository.On("InsertContinuousScreening", mock.Anything, mock.Anything, mock.Anything,
		config, suite.objectType,
		suite.objectId, mock.Anything, mock.Anything).Return(models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                                uuid.New(),
			OrgId:                             uuid.New(),
			ContinuousScreeningConfigId:       suite.configId,
			ContinuousScreeningConfigStableId: suite.configStableId,
			ObjectType:                        suite.objectType,
			ObjectId:                          suite.objectId,
		},
		Matches: []models.ContinuousScreeningMatch{},
	}, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectPayload:  &payload,
	}

	result, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.NotNil(result)
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_TableNotConfigured() {
	// Setup test data - table without FTM entity
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
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
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectId:       &suite.objectId,
	}

	_, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "table is not configured for the use case")
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_ObjectIdNotFoundInIngestedData() {
	// Setup test data
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
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
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table,
		suite.objectId, mock.Anything).Return([]models.DataModelObject{}, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectId:       &suite.objectId,
	}

	_, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "object test-object-id not found in ingested data")
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_ObjectPayloadNotIngested() {
	// Setup test data - payload with object_id
	payload := json.RawMessage(`{"object_id": "test-object-id", "amount": 100}`)

	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
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
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)
	suite.ingestionUsecase.On("IngestObject", mock.Anything, suite.orgId.String(),
		suite.objectType, payload).Return(0, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectPayload:  &payload,
	}

	_, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "no object ingested")
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_UniqueViolationWithIgnoreConflictError() {
	// Setup test data - object payload, which will set ignoreConflictError to true
	payload := json.RawMessage(`{"object_id": "test-object-id", "amount": 100}`)

	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
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

	objectInternalId := uuid.New()
	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
			Metadata: map[string]any{
				"id": [16]byte(objectInternalId),
			},
		},
	}

	// Setup expectations
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)
	suite.ingestionUsecase.On("IngestObject", mock.Anything, suite.orgId.String(),
		suite.objectType, payload).Return(1, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table,
		suite.objectId, mock.Anything).Return(ingestedObjects, nil)
	suite.screeningProvider.On("Search", mock.Anything, mock.MatchedBy(func(query models.OpenSanctionsQuery) bool {
		return len(query.Queries) > 0
	})).Return(models.ScreeningRawSearchResponseWithMatches{
		SearchInput:       []byte("{}"),
		InitialHasMatches: false,
		Matches:           []models.ScreeningMatch{},
	}, nil)
	// Return a unique violation error
	suite.clientDbRepository.On("InsertContinuousScreeningObject", mock.Anything, mock.Anything,
		suite.objectType, suite.objectId, suite.configStableId).Return(&pgconn.PgError{
		Code: pgerrcode.UniqueViolation,
	})
	suite.repository.On("InsertContinuousScreening", mock.Anything, mock.Anything, mock.Anything,
		config, suite.objectType,
		suite.objectId, mock.Anything, mock.Anything).Return(models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                                uuid.New(),
			OrgId:                             uuid.New(),
			ContinuousScreeningConfigId:       suite.configId,
			ContinuousScreeningConfigStableId: suite.configStableId,
			ObjectType:                        suite.objectType,
			ObjectId:                          suite.objectId,
		},
		Matches: []models.ContinuousScreeningMatch{},
	}, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectPayload:  &payload,
	}

	result, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert - should not error when ignoreConflictError is true and unique violation occurs
	suite.NoError(err)
	suite.NotNil(result)
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_UniqueViolationWithoutIgnoreConflictError() {
	// Setup test data - object ID, which will NOT set ignoreConflictError
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
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

	objectInternalId := uuid.New()
	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
			Metadata: map[string]any{
				"id": [16]byte(objectInternalId),
			},
		},
	}

	// Setup expectations
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table,
		suite.objectId, mock.Anything).Return(ingestedObjects, nil)
	suite.clientDbRepository.On("InsertContinuousScreeningObject", mock.Anything, mock.Anything,
		suite.objectType, suite.objectId, suite.configStableId).Return(&pgconn.PgError{
		Code: pgerrcode.UniqueViolation,
	})

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectId:       &suite.objectId,
	}

	_, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert - should error when ignoreConflictError is false and unique violation occurs
	suite.Error(err)
	suite.True(errors.Is(err, models.ConflictError), "error should be ConflictError")
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_ObjectTypeNotConfigured() {
	// Setup test data - config doesn't include the object type
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{"other_table"}, // Config has "other_table" but we're trying to use "transactions"
	}

	// Setup expectations
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType, // "transactions" which is not in ObjectTypes
		ConfigStableId: suite.configStableId,
		ObjectId:       &suite.objectId,
	}

	_, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "object type transactions is not configured with this config")
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_WithMatches_CreatesCase() {
	// Setup test data with config that has inbox
	inboxId := uuid.New()
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
		InboxId:     inboxId,
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

	objectInternalId := uuid.New()
	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
			Metadata: map[string]any{
				"id": [16]byte(objectInternalId),
			},
		},
	}

	// Mock screening provider to return matches (which should result in "in review" status)
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.enforceSecurity.On("UserId").Return(&suite.objectId) // Return a user ID
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table,
		suite.objectId, mock.Anything).Return(ingestedObjects, nil)
	suite.screeningProvider.On("Search", mock.Anything, mock.MatchedBy(func(query models.OpenSanctionsQuery) bool {
		return len(query.Queries) > 0
	})).Return(models.ScreeningRawSearchResponseWithMatches{
		SearchInput:       []byte("{}"),
		InitialHasMatches: true, // Set to true to trigger "in review" status
		Matches: []models.ScreeningMatch{
			{
				EntityId: "test-entity-id",
				Payload:  json.RawMessage(`{"name": "Test Entity"}`),
			},
		},
	}, nil)
	suite.clientDbRepository.On("InsertContinuousScreeningObject", mock.Anything, mock.Anything,
		suite.objectType, suite.objectId, suite.configStableId).Return(nil)
	suite.repository.On("InsertContinuousScreening", mock.Anything, mock.Anything, mock.Anything,
		config, suite.objectType,
		suite.objectId, mock.Anything, mock.Anything).Return(models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                                uuid.New(),
			OrgId:                             uuid.New(),
			ContinuousScreeningConfigId:       suite.configId,
			ContinuousScreeningConfigStableId: suite.configStableId,
			ObjectType:                        suite.objectType,
			ObjectId:                          suite.objectId,
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				ContinuousScreeningId: uuid.New(),
				OpenSanctionEntityId:  "test-entity-id",
				Payload:               json.RawMessage(`{"name": "Test Entity"}`),
			},
		},
	}, nil)

	// Mock case creation
	caseId := uuid.New()
	expectedCase := models.Case{
		Id:             caseId.String(), // Case ID is a string
		Name:           "Continuous Screening - " + suite.objectId,
		InboxId:        inboxId,
		OrganizationId: suite.orgId.String(),
	}
	suite.caseEditor.On("CreateCase", mock.Anything, mock.Anything, suite.objectId, mock.MatchedBy(func(
		attrs models.CreateCaseAttributes,
	) bool {
		return attrs.OrganizationId == suite.orgId.String() &&
			attrs.InboxId == inboxId &&
			attrs.Name == "Continuous Screening - "+suite.objectId &&
			len(attrs.ContinuousScreeningIds) == 1
	}), false).Return(expectedCase, nil)

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectId:       &suite.objectId,
	}

	result, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.NotNil(result)
	suite.AssertExpectations()
}

func (suite *ContinuousScreeningUsecaseTestSuite) TestInsertContinuousScreeningObject_WithMatches_CaseCreationFails() {
	// Setup test data with config that has inbox
	inboxId := uuid.New()
	config := models.ContinuousScreeningConfig{
		Id:          suite.configId,
		StableId:    suite.configStableId,
		OrgId:       suite.orgId,
		ObjectTypes: []string{suite.objectType},
		InboxId:     inboxId,
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

	objectInternalId := uuid.New()
	ingestedObjects := []models.DataModelObject{
		{
			Data: map[string]any{
				"object_id": suite.objectId,
			},
			Metadata: map[string]any{
				"id": [16]byte(objectInternalId),
			},
		},
	}

	// Mock screening provider to return matches
	suite.repository.On("GetContinuousScreeningConfigByStableId", mock.Anything, mock.Anything,
		suite.configStableId).Return(config, nil)
	suite.enforceSecurity.On("WriteContinuousScreeningObject", suite.orgId).Return(nil)
	suite.enforceSecurity.On("UserId").Return(&suite.objectId)
	suite.repository.On("GetDataModel", mock.Anything, mock.Anything, suite.orgId.String(), false, false).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", mock.Anything, mock.Anything, table,
		suite.objectId, mock.Anything).Return(ingestedObjects, nil)
	suite.screeningProvider.On("Search", mock.Anything, mock.MatchedBy(func(query models.OpenSanctionsQuery) bool {
		return len(query.Queries) > 0
	})).Return(models.ScreeningRawSearchResponseWithMatches{
		SearchInput:       []byte("{}"),
		InitialHasMatches: true,
		Matches: []models.ScreeningMatch{
			{
				EntityId: "test-entity-id",
				Payload:  json.RawMessage(`{"name": "Test Entity"}`),
			},
		},
	}, nil)
	suite.clientDbRepository.On("InsertContinuousScreeningObject", mock.Anything, mock.Anything,
		suite.objectType, suite.objectId, suite.configStableId).Return(nil)
	suite.repository.On("InsertContinuousScreening", mock.Anything, mock.Anything, mock.Anything,
		config, suite.objectType,
		suite.objectId, mock.Anything, mock.Anything).Return(models.ContinuousScreeningWithMatches{
		ContinuousScreening: models.ContinuousScreening{
			Id:                                uuid.New(),
			OrgId:                             uuid.New(),
			ContinuousScreeningConfigId:       suite.configId,
			ContinuousScreeningConfigStableId: suite.configStableId,
			ObjectType:                        suite.objectType,
			ObjectId:                          suite.objectId,
		},
		Matches: []models.ContinuousScreeningMatch{
			{
				ContinuousScreeningId: uuid.New(),
				OpenSanctionEntityId:  "test-entity-id",
				Payload:               json.RawMessage(`{"name": "Test Entity"}`),
			},
		},
	}, nil)

	// Mock case creation to fail
	suite.caseEditor.On("CreateCase", mock.Anything, mock.Anything, suite.objectId, mock.MatchedBy(func(
		attrs models.CreateCaseAttributes,
	) bool {
		return attrs.OrganizationId == suite.orgId.String() &&
			attrs.InboxId == inboxId &&
			attrs.Name == "Continuous Screening - "+suite.objectId &&
			len(attrs.ContinuousScreeningIds) == 1
	}), false).Return(models.Case{}, errors.New("case creation failed"))

	// Execute
	uc := suite.makeUsecase()
	input := models.InsertContinuousScreeningObject{
		ObjectType:     suite.objectType,
		ConfigStableId: suite.configStableId,
		ObjectId:       &suite.objectId,
	}

	_, err := uc.InsertContinuousScreeningObject(suite.ctx, input)

	// Assert - should still succeed despite case creation failure (logged as warning)
	suite.Error(err)
	suite.Contains(err.Error(), "case creation failed")
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

func TestStringRepresentation(t *testing.T) {
	// Test with time.Time
	testTime := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	result := stringRepresentation(testTime)
	assert.Equal(t, "2024-01-15T10:30:45Z", result)

	// Test with nil
	result = stringRepresentation(nil)
	assert.Equal(t, "", result)

	// Test with string
	result = stringRepresentation("test-string")
	assert.Equal(t, "test-string", result)

	// Test with integer
	result = stringRepresentation(42)
	assert.Equal(t, "42", result)

	// Test with float
	result = stringRepresentation(3.14)
	assert.Equal(t, "3.14", result)

	// Test with boolean
	result = stringRepresentation(true)
	assert.Equal(t, "true", result)
}

func sortOpenSanctionsFilter(filter models.OpenSanctionsFilter) models.OpenSanctionsFilter {
	sorted := make(models.OpenSanctionsFilter)
	for key, values := range filter {
		sortedValues := make([]string, len(values))
		copy(sortedValues, values)
		sort.Strings(sortedValues)
		sorted[key] = sortedValues
	}
	return sorted
}

func TestPrepareScreeningFilters(t *testing.T) {
	tests := []struct {
		name             string
		ingestedObject   models.DataModelObject
		dataModelMapping map[string]string
		expectedFilters  models.OpenSanctionsFilter
		wantError        bool
		errorContains    string
	}{
		{
			name: "single field mapping",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"name": "John Doe",
				},
			},
			dataModelMapping: map[string]string{
				"name": "name",
			},
			expectedFilters: models.OpenSanctionsFilter{
				"name": []string{"John Doe"},
			},
			wantError: false,
		},
		{
			name: "multiple fields with different properties",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"first_name": "John",
					"country":    "US",
				},
			},
			dataModelMapping: map[string]string{
				"first_name": "name",
				"country":    "country",
			},
			expectedFilters: models.OpenSanctionsFilter{
				"name":    []string{"John"},
				"country": []string{"US"},
			},
			wantError: false,
		},
		{
			name: "multiple fields mapping to same property",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"first_name": "John",
					"last_name":  "Doe",
					"email":      "john@example.com",
				},
			},
			dataModelMapping: map[string]string{
				"first_name": "name",
				"last_name":  "name",
				"email":      "email",
			},
			expectedFilters: models.OpenSanctionsFilter{
				"name":  []string{"John", "Doe"},
				"email": []string{"john@example.com"},
			},
			wantError: false,
		},
		{
			name: "with nil value",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"name": nil,
				},
			},
			dataModelMapping: map[string]string{
				"name": "name",
			},
			expectedFilters: models.OpenSanctionsFilter{
				"name": []string{""},
			},
			wantError: false,
		},
		{
			name: "with timestamp value",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"created_at": time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC),
				},
			},
			dataModelMapping: map[string]string{
				"created_at": "date",
			},
			expectedFilters: models.OpenSanctionsFilter{
				"date": []string{"2024-01-15T10:30:45Z"},
			},
			wantError: false,
		},
		{
			name: "missing field in ingested data",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"name": "John",
				},
			},
			dataModelMapping: map[string]string{
				"name":    "name",
				"country": "country",
			},
			wantError:     true,
			errorContains: "field country not found in ingested object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := prepareScreeningFilters(tt.ingestedObject, tt.dataModelMapping)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, sortOpenSanctionsFilter(tt.expectedFilters), sortOpenSanctionsFilter(result))
			}
		})
	}
}

func TestPrepareOpenSanctionsQuery(t *testing.T) {
	tests := []struct {
		name                string
		ingestedObject      models.DataModelObject
		dataModelEntityType string
		dataModelMapping    map[string]string
		config              models.ContinuousScreeningConfig
		expectedQuery       models.OpenSanctionsQuery
		wantError           bool
		errorContains       string
	}{
		{
			name: "valid query with single filter",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"name": "John Doe",
				},
			},
			dataModelEntityType: "Person",
			dataModelMapping: map[string]string{
				"name": "name",
			},
			config: models.ContinuousScreeningConfig{
				MatchThreshold: 75,
				MatchLimit:     10,
				Datasets:       []string{"default"},
			},
			expectedQuery: models.OpenSanctionsQuery{
				OrgConfig: models.OrganizationOpenSanctionsConfig{
					MatchThreshold: 75,
					MatchLimit:     10,
				},
				Config: models.ScreeningConfig{
					Datasets: []string{"default"},
				},
				Queries: []models.OpenSanctionsCheckQuery{
					{
						Type: "Person",
						Filters: models.OpenSanctionsFilter{
							"name": []string{"John Doe"},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "valid query with multiple filters",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"first_name": "John",
					"last_name":  "Doe",
					"country":    "US",
				},
			},
			dataModelEntityType: "Person",
			dataModelMapping: map[string]string{
				"first_name": "name",
				"last_name":  "name",
				"country":    "country",
			},
			config: models.ContinuousScreeningConfig{
				MatchThreshold: 80,
				MatchLimit:     20,
				Datasets:       []string{"default", "custom"},
			},
			expectedQuery: models.OpenSanctionsQuery{
				OrgConfig: models.OrganizationOpenSanctionsConfig{
					MatchThreshold: 80,
					MatchLimit:     20,
				},
				Config: models.ScreeningConfig{
					Datasets: []string{"default", "custom"},
				},
				Queries: []models.OpenSanctionsCheckQuery{
					{
						Type: "Person",
						Filters: models.OpenSanctionsFilter{
							"name":    []string{"John", "Doe"},
							"country": []string{"US"},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "entity type Company",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"company_name": "Acme Corp",
				},
			},
			dataModelEntityType: "Company",
			dataModelMapping: map[string]string{
				"company_name": "name",
			},
			config: models.ContinuousScreeningConfig{
				MatchThreshold: 70,
				MatchLimit:     5,
				Datasets:       []string{"default"},
			},
			expectedQuery: models.OpenSanctionsQuery{
				OrgConfig: models.OrganizationOpenSanctionsConfig{
					MatchThreshold: 70,
					MatchLimit:     5,
				},
				Config: models.ScreeningConfig{
					Datasets: []string{"default"},
				},
				Queries: []models.OpenSanctionsCheckQuery{
					{
						Type: "Company",
						Filters: models.OpenSanctionsFilter{
							"name": []string{"Acme Corp"},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "missing field in ingested data",
			ingestedObject: models.DataModelObject{
				Data: map[string]any{
					"name": "John",
				},
			},
			dataModelEntityType: "Person",
			dataModelMapping: map[string]string{
				"name":    "name",
				"country": "country",
			},
			config: models.ContinuousScreeningConfig{
				MatchThreshold: 75,
				MatchLimit:     10,
				Datasets:       []string{"default"},
			},
			wantError:     true,
			errorContains: "field country not found in ingested object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := prepareOpenSanctionsQuery(tt.ingestedObject,
				tt.dataModelEntityType, tt.dataModelMapping, tt.config)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedQuery.OrgConfig, result.OrgConfig)
				assert.Equal(t, tt.expectedQuery.Config.Datasets, result.Config.Datasets)
				assert.Equal(t, len(tt.expectedQuery.Queries), len(result.Queries))
				if len(result.Queries) > 0 {
					assert.Equal(t, tt.expectedQuery.Queries[0].Type, result.Queries[0].Type)
					assert.Equal(t, tt.expectedQuery.Queries[0].Filters, result.Queries[0].Filters)
				}
			}
		})
	}
}

func TestBuildDataModelMapping(t *testing.T) {
	ftmEntity := models.FollowTheMoneyEntityPerson
	ftmEntityCompany := models.FollowTheMoneyEntityCompany
	ftmPropertyName := models.FollowTheMoneyPropertyName
	ftmPropertyCountry := models.FollowTheMoneyPropertyCountry
	ftmPropertyAddress := models.FollowTheMoneyPropertyAddress

	tests := []struct {
		name            string
		table           models.Table
		expectedMapping models.ContinuousScreeningDataModelMapping
		wantError       bool
		errorMsg        string
	}{
		{
			name: "single field with FTM property",
			table: models.Table{
				Name:      "customers",
				FTMEntity: &ftmEntity,
				Fields: map[string]models.Field{
					"customer_name": {
						Name:        "customer_name",
						FTMProperty: &ftmPropertyName,
					},
				},
			},
			expectedMapping: models.ContinuousScreeningDataModelMapping{
				Entity: "Person",
				Properties: map[string]string{
					"customer_name": "name",
				},
			},
			wantError: false,
		},
		{
			name: "multiple fields with different FTM properties",
			table: models.Table{
				Name:      "customers",
				FTMEntity: &ftmEntity,
				Fields: map[string]models.Field{
					"first_name": {
						Name:        "first_name",
						FTMProperty: &ftmPropertyName,
					},
					"country_code": {
						Name:        "country_code",
						FTMProperty: &ftmPropertyCountry,
					},
				},
			},
			expectedMapping: models.ContinuousScreeningDataModelMapping{
				Entity: "Person",
				Properties: map[string]string{
					"first_name":   "name",
					"country_code": "country",
				},
			},
			wantError: false,
		},
		{
			name: "Company entity with address property",
			table: models.Table{
				Name:      "companies",
				FTMEntity: &ftmEntityCompany,
				Fields: map[string]models.Field{
					"company_name": {
						Name:        "company_name",
						FTMProperty: &ftmPropertyName,
					},
					"company_address": {
						Name:        "company_address",
						FTMProperty: &ftmPropertyAddress,
					},
				},
			},
			expectedMapping: models.ContinuousScreeningDataModelMapping{
				Entity: "Company",
				Properties: map[string]string{
					"company_name":    "name",
					"company_address": "address",
				},
			},
			wantError: false,
		},
		{
			name: "fields with and without FTM property (only mapped fields included)",
			table: models.Table{
				Name:      "customers",
				FTMEntity: &ftmEntity,
				Fields: map[string]models.Field{
					"customer_name": {
						Name:        "customer_name",
						FTMProperty: &ftmPropertyName,
					},
					"email": {
						Name:        "email",
						FTMProperty: nil,
					},
				},
			},
			expectedMapping: models.ContinuousScreeningDataModelMapping{
				Entity: "Person",
				Properties: map[string]string{
					"customer_name": "name",
				},
			},
			wantError: false,
		},
		{
			name: "missing FTM entity",
			table: models.Table{
				Name:      "customers",
				FTMEntity: nil,
				Fields: map[string]models.Field{
					"customer_name": {
						Name:        "customer_name",
						FTMProperty: &ftmPropertyName,
					},
				},
			},
			wantError: true,
			errorMsg:  "table is not configured for the use case",
		},
		{
			name: "no fields with FTM property",
			table: models.Table{
				Name:      "customers",
				FTMEntity: &ftmEntity,
				Fields: map[string]models.Field{
					"email": {
						Name:        "email",
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
			result, err := buildDataModelMapping(tt.table)

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMapping.Entity, result.Entity)
				assert.Equal(t, tt.expectedMapping.Properties, result.Properties)
			}
		})
	}
}
