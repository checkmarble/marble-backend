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

type ObjectMetadataUsecaseTestSuite struct {
	suite.Suite
	enforceSecurity    *mocks.EnforceSecurity
	repository         *mocks.ObjectMetadataRepository
	ingestedDataReader *mocks.IngestedDataReader
	exec               *mocks.Executor
	transaction        *mocks.Transaction
	executorFactory    *mocks.ExecutorFactory

	ctx   context.Context
	orgId uuid.UUID
}

func (suite *ObjectMetadataUsecaseTestSuite) SetupTest() {
	suite.enforceSecurity = new(mocks.EnforceSecurity)
	suite.repository = new(mocks.ObjectMetadataRepository)
	suite.ingestedDataReader = new(mocks.IngestedDataReader)
	suite.exec = new(mocks.Executor)
	suite.transaction = new(mocks.Transaction)
	suite.executorFactory = new(mocks.ExecutorFactory)

	suite.ctx = context.Background()
	suite.orgId = uuid.MustParse("12345678-1234-1234-1234-123456789012")
}

func (suite *ObjectMetadataUsecaseTestSuite) makeUsecase() *ObjectMetadataUsecase {
	return &ObjectMetadataUsecase{
		executorFactory:    suite.executorFactory,
		enforceSecurity:    suite.enforceSecurity,
		repository:         suite.repository,
		ingestedDataReader: suite.ingestedDataReader,
	}
}

func TestObjectMetadataUsecaseTestSuite(t *testing.T) {
	suite.Run(t, new(ObjectMetadataUsecaseTestSuite))
}

// =============================================================================
// UpsertObjectRiskTopic Tests
// =============================================================================

func (suite *ObjectMetadataUsecaseTestSuite) TestUpsertObjectRiskTopic_TableNotFound() {
	// Setup
	input := models.ObjectRiskTopicUpsert{
		OrgId:      suite.orgId,
		ObjectType: "non_existent_table",
		ObjectId:   "obj-123",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions},
	}

	// DataModel without the requested table
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"other_table": {Name: "other_table"},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteObjectMetadata", suite.orgId).Return(nil)
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

func (suite *ObjectMetadataUsecaseTestSuite) TestUpsertObjectRiskTopic_IngestedObjectNotFound() {
	// Setup
	input := models.ObjectRiskTopicUpsert{
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "non-existent-user",
		Topics:     []models.RiskTopic{models.RiskTopicSanctions},
	}

	table := models.Table{Name: "users"}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"users": table,
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteObjectMetadata", suite.orgId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.orgId).Return(suite.exec, nil)
	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.repository.On("GetDataModel", suite.ctx, suite.exec, suite.orgId, false, true).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, suite.exec, table, "non-existent-user",
		mock.Anything).Return([]models.DataModelObject{}, nil)

	// Execute
	uc := suite.makeUsecase()
	_, err := uc.UpsertObjectRiskTopic(suite.ctx, input)

	// Assert
	suite.Error(err)
	suite.Contains(err.Error(), "ingested object not found")
}

func (suite *ObjectMetadataUsecaseTestSuite) TestUpsertObjectRiskTopic_HappyPath() {
	// Setup
	objectMetadataId := uuid.New()
	input := models.ObjectRiskTopicUpsert{
		OrgId:         suite.orgId,
		ObjectType:    "users",
		ObjectId:      "user-123",
		Topics:        []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
		SourceType:    models.RiskTopicSourceTypeManual,
		SourceDetails: nil,
	}

	table := models.Table{Name: "users"}
	dataModel := models.DataModel{
		Tables: map[string]models.Table{
			"users": table,
		},
	}

	expectedObjectMetadata := models.ObjectMetadata{
		Id:           objectMetadataId,
		OrgId:        suite.orgId,
		ObjectType:   "users",
		ObjectId:     "user-123",
		MetadataType: models.MetadataTypeRiskTopics,
		Metadata: models.RiskTopicsMetadata{
			Topics: []models.RiskTopic{models.RiskTopicPEPs, models.RiskTopicSanctions},
		},
	}

	// Mock expectations
	suite.enforceSecurity.On("WriteObjectMetadata", suite.orgId).Return(nil)
	suite.executorFactory.On("NewClientDbExecutor", suite.ctx, suite.orgId).Return(suite.exec, nil)
	suite.executorFactory.On("NewExecutor").Return(suite.exec)
	suite.repository.On("GetDataModel", suite.ctx, suite.exec, suite.orgId, false, true).Return(dataModel, nil)
	suite.ingestedDataReader.On("QueryIngestedObject", suite.ctx, suite.exec, table, "user-123",
		mock.Anything).Return([]models.DataModelObject{{Data: map[string]any{"id": "user-123"}}}, nil)

	// Expect UpsertObjectMetadata to be called with correct data
	suite.repository.On("UpsertObjectMetadata", suite.ctx, suite.exec,
		mock.MatchedBy(func(upsert models.ObjectMetadataUpsert) bool {
			metadata, ok := upsert.Metadata.(models.RiskTopicsMetadata)
			if !ok {
				return false
			}
			return upsert.OrgId == suite.orgId &&
				upsert.ObjectType == "users" &&
				upsert.ObjectId == "user-123" &&
				upsert.MetadataType == models.MetadataTypeRiskTopics &&
				len(metadata.Topics) == 2 &&
				slices.Contains(metadata.Topics, models.RiskTopicSanctions) &&
				slices.Contains(metadata.Topics, models.RiskTopicPEPs)
		})).Return(expectedObjectMetadata, nil)

	// Execute
	uc := suite.makeUsecase()
	result, err := uc.UpsertObjectRiskTopic(suite.ctx, input)

	// Assert
	suite.NoError(err)
	suite.Equal(expectedObjectMetadata, result)
}

// =============================================================================
// AppendObjectRiskTopics Tests
// =============================================================================

func (suite *ObjectMetadataUsecaseTestSuite) TestAppendObjectRiskTopics_ObjectNotExists_SavesNewTopics() {
	// Setup - Object metadata doesn't exist yet
	objectMetadataId := uuid.New()
	input := models.ObjectRiskTopicUpsert{
		OrgId:         suite.orgId,
		ObjectType:    "users",
		ObjectId:      "user-123",
		Topics:        []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
		SourceType:    models.RiskTopicSourceTypeContinuousScreeningMatchReview,
		SourceDetails: nil,
	}

	expectedObjectMetadata := models.ObjectMetadata{
		Id:           objectMetadataId,
		OrgId:        suite.orgId,
		ObjectType:   "users",
		ObjectId:     "user-123",
		MetadataType: models.MetadataTypeRiskTopics,
		Metadata: models.RiskTopicsMetadata{
			Topics: []models.RiskTopic{models.RiskTopicPEPs, models.RiskTopicSanctions},
		},
	}

	// Mock expectations - GetObjectMetadata returns NotFoundError
	suite.repository.On("GetObjectMetadata", suite.ctx, suite.transaction,
		suite.orgId, "users", "user-123", models.MetadataTypeRiskTopics).
		Return(models.ObjectMetadata{Metadata: models.RiskTopicsMetadata{}}, models.NotFoundError)

	// Expect UpsertObjectMetadata to be called with the new topics
	suite.repository.On("UpsertObjectMetadata", suite.ctx, suite.transaction,
		mock.MatchedBy(func(upsert models.ObjectMetadataUpsert) bool {
			metadata, ok := upsert.Metadata.(models.RiskTopicsMetadata)
			if !ok {
				return false
			}
			return upsert.OrgId == suite.orgId &&
				upsert.ObjectType == "users" &&
				upsert.ObjectId == "user-123" &&
				len(metadata.Topics) == 2 &&
				slices.Contains(metadata.Topics, models.RiskTopicSanctions) &&
				slices.Contains(metadata.Topics, models.RiskTopicPEPs)
		})).Return(expectedObjectMetadata, nil)

	// Execute
	uc := suite.makeUsecase()
	err := uc.AppendObjectRiskTopics(suite.ctx, suite.transaction, input)

	// Assert
	suite.NoError(err)
	suite.repository.AssertExpectations(suite.T())
}

func (suite *ObjectMetadataUsecaseTestSuite) TestAppendObjectRiskTopics_ObjectExists_MergesTopics() {
	// Setup - Object metadata already exists with some topics
	objectMetadataId := uuid.New()
	existingObjectMetadata := models.ObjectMetadata{
		Id:           objectMetadataId,
		OrgId:        suite.orgId,
		ObjectType:   "users",
		ObjectId:     "user-123",
		MetadataType: models.MetadataTypeRiskTopics,
		Metadata: models.RiskTopicsMetadata{
			Topics: []models.RiskTopic{models.RiskTopicSanctions}, // Existing topic
		},
	}

	input := models.ObjectRiskTopicUpsert{
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		// Adding Sanctions (already exists) and PEPs (new)
		Topics:        []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
		SourceType:    models.RiskTopicSourceTypeContinuousScreeningMatchReview,
		SourceDetails: nil,
	}

	updatedObjectMetadata := models.ObjectMetadata{
		Id:           objectMetadataId,
		OrgId:        suite.orgId,
		ObjectType:   "users",
		ObjectId:     "user-123",
		MetadataType: models.MetadataTypeRiskTopics,
		Metadata: models.RiskTopicsMetadata{
			Topics: []models.RiskTopic{models.RiskTopicPEPs, models.RiskTopicSanctions}, // Merged
		},
	}

	// Mock expectations - GetObjectMetadata returns existing record
	suite.repository.On("GetObjectMetadata", suite.ctx, suite.transaction,
		suite.orgId, "users", "user-123", models.MetadataTypeRiskTopics).
		Return(existingObjectMetadata, nil)

	// Expect UpsertObjectMetadata to be called with MERGED topics (Sanctions + PEPs)
	suite.repository.On("UpsertObjectMetadata", suite.ctx, suite.transaction,
		mock.MatchedBy(func(upsert models.ObjectMetadataUpsert) bool {
			metadata, ok := upsert.Metadata.(models.RiskTopicsMetadata)
			if !ok {
				return false
			}
			return upsert.OrgId == suite.orgId &&
				upsert.ObjectType == "users" &&
				upsert.ObjectId == "user-123" &&
				len(metadata.Topics) == 2 &&
				slices.Contains(metadata.Topics, models.RiskTopicSanctions) &&
				slices.Contains(metadata.Topics, models.RiskTopicPEPs)
		})).Return(updatedObjectMetadata, nil)

	// Execute
	uc := suite.makeUsecase()
	err := uc.AppendObjectRiskTopics(suite.ctx, suite.transaction, input)

	// Assert
	suite.NoError(err)
	suite.repository.AssertExpectations(suite.T())
}

func (suite *ObjectMetadataUsecaseTestSuite) TestAppendObjectRiskTopics_NoNewTopics_SkipsUpsert() {
	// Setup - Object metadata already exists with the same topics
	objectMetadataId := uuid.New()
	existingObjectMetadata := models.ObjectMetadata{
		Id:           objectMetadataId,
		OrgId:        suite.orgId,
		ObjectType:   "users",
		ObjectId:     "user-123",
		MetadataType: models.MetadataTypeRiskTopics,
		Metadata: models.RiskTopicsMetadata{
			Topics: []models.RiskTopic{models.RiskTopicSanctions, models.RiskTopicPEPs},
		},
	}

	input := models.ObjectRiskTopicUpsert{
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		// Same topics as existing - no new topics
		Topics:     []models.RiskTopic{models.RiskTopicSanctions},
		SourceType: models.RiskTopicSourceTypeContinuousScreeningMatchReview,
	}

	// Mock expectations - GetObjectMetadata returns existing record
	suite.repository.On("GetObjectMetadata", suite.ctx, suite.transaction,
		suite.orgId, "users", "user-123", models.MetadataTypeRiskTopics).
		Return(existingObjectMetadata, nil)

	// UpsertObjectMetadata should NOT be called because there are no new topics to add

	// Execute
	uc := suite.makeUsecase()
	err := uc.AppendObjectRiskTopics(suite.ctx, suite.transaction, input)

	// Assert
	suite.NoError(err)
}

func (suite *ObjectMetadataUsecaseTestSuite) TestAppendObjectRiskTopics_ConfirmedHit_TopicAlreadyExists_SkipsUpsert() {
	// Setup - Object metadata already exists with the match topic from a previous confirmed hit
	objectMetadataId := uuid.New()
	existingObjectMetadata := models.ObjectMetadata{
		Id:           objectMetadataId,
		OrgId:        suite.orgId,
		ObjectType:   "users",
		ObjectId:     "user-123",
		MetadataType: models.MetadataTypeRiskTopics,
		Metadata: models.RiskTopicsMetadata{
			Topics: []models.RiskTopic{models.RiskTopicSanctions}, // Already has Sanctions from previous confirmed hit
		},
	}

	input := models.ObjectRiskTopicUpsert{
		OrgId:      suite.orgId,
		ObjectType: "users",
		ObjectId:   "user-123",
		// Trying to add Sanctions again from a new confirmed hit - should be a no-op
		Topics:     []models.RiskTopic{models.RiskTopicSanctions},
		SourceType: models.RiskTopicSourceTypeContinuousScreeningMatchReview,
	}

	// Mock expectations - GetObjectMetadata returns existing record with matching topic
	suite.repository.On("GetObjectMetadata", suite.ctx, suite.transaction,
		suite.orgId, "users", "user-123", models.MetadataTypeRiskTopics).
		Return(existingObjectMetadata, nil)

	// Execute
	uc := suite.makeUsecase()
	err := uc.AppendObjectRiskTopics(suite.ctx, suite.transaction, input)

	// Assert
	suite.NoError(err)

	// UpsertObjectMetadata should NOT be called
}
