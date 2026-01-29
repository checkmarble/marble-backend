package usecases

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
)

type ObjectRiskTopicUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity    *mocks.EnforceSecurity
	repository         *mocks.ObjectRiskTopicRepository
	ingestedDataReader *mocks.IngestedDataReader
	exec               *mocks.Executor
	transaction        *mocks.Transaction
	transactionFactory *mocks.TransactionFactory
	executorFactory    *mocks.ExecutorFactory

	ctx    context.Context
	orgId  uuid.UUID
	userId uuid.UUID
}

func (suite *ObjectRiskTopicUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.repository = new(mocks.ObjectRiskTopicRepository)
	suite.ingestedDataReader = new(mocks.IngestedDataReader)
	suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.transactionFactory = &mocks.TransactionFactory{TxMock: suite.transaction}
	suite.executorFactory = new(mocks.ExecutorFactory)

	suite.ctx = context.Background()
	suite.orgId = uuid.MustParse("12345678-1234-1234-1234-123456789012")
	suite.userId = uuid.MustParse("abcdefab-1234-1234-1234-123456789012")
}

func (suite *ObjectRiskTopicUsecaseTestSuite) makeUsecase() *ObjectRiskTopicUsecase {
	return &ObjectRiskTopicUsecase{
		executorFactory:    suite.executorFactory,
		transactionFactory: suite.transactionFactory,
		enforceSecurity:    suite.enforceSecurity,
		repository:         suite.repository,
		ingestedDataReader: suite.ingestedDataReader,
	}
}

func TestObjectRiskTopicUsecaseTestSuite(t *testing.T) {
	suite.Run(t, new(ObjectRiskTopicUsecaseTestSuite))
}

// =============================================================================
// UpsertObjectRiskTopic Tests
// =============================================================================

func (suite *ObjectRiskTopicUsecaseTestSuite) TestUpsertObjectRiskTopic_TableNotFound() {
	// Setup
	input := models.ObjectRiskTopicWithEventUpsert{
		OrgId:      suite.orgId,
		ObjectType: "non_existent_table",
		ObjectId:   "obj-123",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions},
		UserId:     suite.userId,
	}

	// DataModel without the requested table
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"other_table": {Name: "other_table"},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteObjectRiskTopic", suite.orgId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.orgId).Return(suite.exec, nil)
	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.repository.On("GetDataModel", suite.ctx, suite.exec, suite.orgId, false, true).Return(dataModel, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpsertObjectRiskTopic(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.True(errors.Is(err, models.BadParameterError))
	suite.Contains(err.Error(), "table non_existent_table not found in data model")
}

func (suite *ObjectRiskTopicUsecaseTestSuite) TestUpsertObjectRiskTopic_IngestedObjectNotFound() {
	// Setup
	input := models.ObjectRiskTopicWithEventUpsert{
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "non-existent-user",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions},
		UserId:     suite.userId,
	}

	table := models.Table{Name: "users"}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"users": table,
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteObjectRiskTopic", suite.orgId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.orgId).Return(suite.exec, nil)
	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.repository.On("GetDataModel", suite.ctx, suite.exec, suite.orgId, false, true).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, suite.exec, table, "non-existent-user",
		mock.Anything).Return([]models.DataModelObject{}, models.NotFoundError)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpsertObjectRiskTopic(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "failed to fetch ingested object")
}

func (suite *ObjectRiskTopicUsecaseTestSuite) TestUpsertObjectRiskTopic_HappyPath() {
	// Setup
	objectRiskTopicId := uuid.New()
	input := models.ObjectRiskTopicWithEventUpsert{
		OrgId:         suite.orgId,
		ObjectType:    "users",
		ObjectId:      "user-123",
		Topics:        []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
		SourceType:    models.RiskTopicSourceTypeManual,
		SourceDetails: nil,
		UserId:        suite.userId,
	}

	table := models.Table{Name: "users"}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"users": table,
		},
	}

	expectedObjectRiskTopic := models.ObjectRiskTopic{
		Id:         objectRiskTopicId,
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteObjectRiskTopic", suite.orgId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.orgId).Return(suite.exec, nil)
	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.repository.On("GetDataModel", suite.ctx, suite.exec, suite.orgId, false, true).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, suite.exec, table, "user-123",
		mock.Anything).Return([]models.DataModelObject{{Data: map[string]any{"id": "user-123"}}}, nil)
	suite.transactionFactory.On("Transaction", suite.ctx, mock.Anything).Return(nil)

	// Expect UpsertObjectRiskTopic to be called with correct data
	suite.repository.On("UpsertObjectRiskTopic", suite.ctx, suite.transaction,
		mock.MatchedBy(func(create models.ObjectRiskTopicCreate) bool {
			return create.OrgId == suite.orgId &&
				create.ObjectType == "users" &&
				create.ObjectId == "user-123" &&
				len(create.Topics) == 2 &&
				slices.Contains(create.Topics, models.RiskTopicSanctions) &&
				slices.Contains(create.Topics, models.RiskTopicPEPs)
		})).Return(expectedObjectRiskTopic, nil)

	// Expect InsertObjectRiskTopicEvent to be called with correct data
	suite.repository.On("InsertObjectRiskTopicEvent", suite.ctx, suite.transaction,
		mock.MatchedBy(func(event models.ObjectRiskTopicEventCreate) bool {
			return event.OrgId == suite.orgId &&
				event.ObjectRiskTopicsId == objectRiskTopicId &&
				len(event.Topics) == 2 &&
				slices.Contains(event.Topics, models.RiskTopicSanctions) &&
				slices.Contains(event.Topics, models.RiskTopicPEPs) &&
				event.SourceType == models.RiskTopicSourceTypeManual &&
				event.UserId != nil && *event.UserId == suite.userId
		})).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpsertObjectRiskTopic(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(expectedObjectRiskTopic, result)
}

// =============================================================================
// AppendObjectRiskTopics Tests
// =============================================================================

func (suite *ObjectRiskTopicUsecaseTestSuite) TestAppendObjectRiskTopics_ObjectNotExists_SavesNewTopics() {
	// Setup - Object risk topic doesn't exist yet
	objectRiskTopicId := uuid.New()
	input := models.ObjectRiskTopicWithEventUpsert{
		OrgId:         suite.orgId,
		ObjectType:    "users",
		ObjectId:      "user-123",
		Topics:        []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
		SourceType:    models.RiskTopicSourceTypeContinuousScreeningMatchReview,
		SourceDetails: nil,
		UserId:        suite.userId,
	}

	expectedObjectRiskTopic := models.ObjectRiskTopic{
		Id:         objectRiskTopicId,
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
	}

	// Mock expectations - GetObjectRiskTopicByObjectId returns NotFoundError
	suite.repository.On("GetObjectRiskTopicByObjectId", suite.ctx, suite.transaction,
		suite.orgId, "users", "user-123").Return(models.ObjectRiskTopic{}, models.NotFoundError)

	// Expect UpsertObjectRiskTopic to be called with the new topics
	suite.repository.On("UpsertObjectRiskTopic", suite.ctx, suite.transaction,
		mock.MatchedBy(func(create models.ObjectRiskTopicCreate) bool {
			return create.OrgId == suite.orgId &&
				create.ObjectType == "users" &&
				create.ObjectId == "user-123" &&
				len(create.Topics) == 2 &&
				slices.Contains(create.Topics, models.RiskTopicSanctions) &&
				slices.Contains(create.Topics, models.RiskTopicPEPs)
		})).Return(expectedObjectRiskTopic, nil)

	// Expect InsertObjectRiskTopicEvent to be called with ALL topics as new
	suite.repository.On("InsertObjectRiskTopicEvent", suite.ctx, suite.transaction,
		mock.MatchedBy(func(event models.ObjectRiskTopicEventCreate) bool {
			return event.OrgId == suite.orgId &&
				event.ObjectRiskTopicsId == objectRiskTopicId &&
				len(event.Topics) == 2 &&
				slices.Contains(event.Topics, models.RiskTopicSanctions) &&
				slices.Contains(event.Topics, models.RiskTopicPEPs) &&
				event.SourceType == models.RiskTopicSourceTypeContinuousScreeningMatchReview
		})).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	err := uc.AppendObjectRiskTopics(suite.ctx, suite.transaction, input)

	// Assert
	suite.NoError(err)
	suite.repository.AssertExpectations(suite.T())
}

func (suite *ObjectRiskTopicUsecaseTestSuite) TestAppendObjectRiskTopics_ObjectExists_MergesTopics() {
	// Setup - Object risk topic already exists with some topics
	objectRiskTopicId := uuid.New()
	existingObjectRiskTopic := models.ObjectRiskTopic{
		Id:         objectRiskTopicId,
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions}, // Existing topic
	}

	input := models.ObjectRiskTopicWithEventUpsert{
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		// Adding Sanctions (already exists) and PEPs (new)
		Topics:        []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
		SourceType:    models.RiskTopicSourceTypeContinuousScreeningMatchReview,
		SourceDetails: nil,
		UserId:        suite.userId,
	}

	updatedObjectRiskTopic := models.ObjectRiskTopic{
		Id:         objectRiskTopicId,
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs}, // Merged
	}

	// Mock expectations - GetObjectRiskTopicByObjectId returns existing record
	suite.repository.On("GetObjectRiskTopicByObjectId", suite.ctx, suite.transaction,
		suite.orgId, "users", "user-123").Return(existingObjectRiskTopic, nil)

	// Expect UpsertObjectRiskTopic to be called with MERGED topics (Sanctions + PEPs)
	suite.repository.On("UpsertObjectRiskTopic", suite.ctx, suite.transaction,
		mock.MatchedBy(func(create models.ObjectRiskTopicCreate) bool {
			return create.OrgId == suite.orgId &&
				create.ObjectType == "users" &&
				create.ObjectId == "user-123" &&
				len(create.Topics) == 2 &&
				slices.Contains(create.Topics, models.RiskTopicSanctions) &&
				slices.Contains(create.Topics, models.RiskTopicPEPs)
		})).Return(updatedObjectRiskTopic, nil)

	// Expect InsertObjectRiskTopicEvent to be called with ONLY NEW topics (just PEPs)
	suite.repository.On("InsertObjectRiskTopicEvent", suite.ctx, suite.transaction,
		mock.MatchedBy(func(event models.ObjectRiskTopicEventCreate) bool {
			return event.OrgId == suite.orgId &&
				event.ObjectRiskTopicsId == objectRiskTopicId &&
				len(event.Topics) == 1 &&
				slices.Contains(event.Topics, models.RiskTopicPEPs) &&
				!slices.Contains(event.Topics, models.RiskTopicSanctions) && // Sanctions should NOT be in event
				event.SourceType == models.RiskTopicSourceTypeContinuousScreeningMatchReview
		})).Return(nil)

	// Execute
	uc := suite.makeUsecase()
	err := uc.AppendObjectRiskTopics(suite.ctx, suite.transaction, input)

	// Assert
	suite.NoError(err)
	suite.repository.AssertExpectations(suite.T())
}

func (suite *ObjectRiskTopicUsecaseTestSuite) TestAppendObjectRiskTopics_NoNewTopics_SkipsUpsert() {
	// Setup - Object risk topic already exists with the same topics
	objectRiskTopicId := uuid.New()
	existingObjectRiskTopic := models.ObjectRiskTopic{
		Id:         objectRiskTopicId,
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
	}

	input := models.ObjectRiskTopicWithEventUpsert{
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		// Same topics as existing - no new topics
		Topics:     []models.RiskTopic{models.RiskTopicSanctions},
		SourceType: models.RiskTopicSourceTypeContinuousScreeningMatchReview,
		UserId:     suite.userId,
	}

	// Mock expectations - GetObjectRiskTopicByObjectId returns existing record
	suite.repository.On("GetObjectRiskTopicByObjectId", suite.ctx, suite.transaction,
		suite.orgId, "users", "user-123").Return(existingObjectRiskTopic, nil)

	// UpsertObjectRiskTopic and InsertObjectRiskTopicEvent should NOT be called
	// because there are no new topics to add

	// Execute
	uc := suite.makeUsecase()
	err := uc.AppendObjectRiskTopics(suite.ctx, suite.transaction, input)

	// Assert
	suite.NoError(err)
}
