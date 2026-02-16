package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for interfaces specific to MonitoringListCheck

type mockMonitoringListCheckRepository struct {
	mock.Mock
}

func (m *mockMonitoringListCheckRepository) FindObjectRiskTopicsMetadata(
	ctx context.Context,
	exec repositories.Executor,
	filter models.ObjectRiskTopicsMetadataFilter,
) ([]models.ObjectMetadata, error) {
	args := m.Called(ctx, exec, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ObjectMetadata), args.Error(1)
}

func (m *mockMonitoringListCheckRepository) ListPivots(
	ctx context.Context,
	exec repositories.Executor,
	organizationId uuid.UUID,
	tableId *string,
	useCache bool,
) ([]models.PivotMetadata, error) {
	args := m.Called(ctx, exec, organizationId, tableId, useCache)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PivotMetadata), args.Error(1)
}

// Helper to build config named args
func buildMonitoringListCheckArgs(config ast.MonitoringListCheckConfig) ast.Arguments {
	configMap := map[string]any{
		"targetTableName": config.TargetTableName,
		"pathToTarget":    config.PathToTarget,
		"topicFilters":    config.TopicFilters,
	}

	if len(config.LinkedTableChecks) > 0 {
		linkedChecks := make([]any, len(config.LinkedTableChecks))
		for i, check := range config.LinkedTableChecks {
			checkMap := map[string]any{
				"tableName": check.TableName,
			}
			if check.LinkToSingleName != nil {
				checkMap["linkToSingleName"] = *check.LinkToSingleName
			}
			if check.NavigationOption != nil {
				checkMap["navigationOption"] = map[string]any{
					"sourceTableName":   check.NavigationOption.SourceTableName,
					"sourceFieldName":   check.NavigationOption.SourceFieldName,
					"targetTableName":   check.NavigationOption.TargetTableName,
					"targetFieldName":   check.NavigationOption.TargetFieldName,
					"orderingFieldName": check.NavigationOption.OrderingFieldName,
				}
			}
			linkedChecks[i] = checkMap
		}
		configMap["linkedTableChecks"] = linkedChecks
	}

	return ast.Arguments{
		NamedArgs: map[string]any{
			"config": configMap,
		},
	}
}

func TestValidateMonitoringListCheckConfig(t *testing.T) {
	tests := []struct {
		name             string
		config           ast.MonitoringListCheckConfig
		expectedErrorLen int
		expectedErrors   []error
	}{
		{
			name: "happy path with LinkToSingleName",
			config: ast.MonitoringListCheckConfig{
				TargetTableName: "users",
				LinkedTableChecks: []ast.LinkedTableCheck{
					{
						TableName:        "accounts",
						LinkToSingleName: utils.Ptr("account_id"),
					},
				},
			},
			expectedErrorLen: 0,
		},
		{
			name: "happy path with NavigationOption",
			config: ast.MonitoringListCheckConfig{
				TargetTableName: "users",
				LinkedTableChecks: []ast.LinkedTableCheck{
					{
						TableName: "accounts",
						NavigationOption: &ast.NavigationOption{
							SourceTableName:   "source",
							SourceFieldName:   "source_id",
							TargetTableName:   "target",
							TargetFieldName:   "target_id",
							OrderingFieldName: "updated_at",
						},
					},
				},
			},
			expectedErrorLen: 0,
		},
		{
			name: "failure case with multiple validation errors",
			config: ast.MonitoringListCheckConfig{
				TargetTableName: "", // missing
				LinkedTableChecks: []ast.LinkedTableCheck{
					{
						TableName:        "", // missing
						LinkToSingleName: nil,
						NavigationOption: nil, // neither present
					},
					{
						TableName:        "accounts",
						LinkToSingleName: utils.Ptr("account_id"),
						NavigationOption: &ast.NavigationOption{ // both present
							SourceTableName:   "source",
							SourceFieldName:   "source_id",
							TargetTableName:   "target",
							TargetFieldName:   "target_id",
							OrderingFieldName: "updated_at",
						},
					},
					{
						TableName: "accounts",
						NavigationOption: &ast.NavigationOption{ // missing all fields
							SourceTableName:   "",
							SourceFieldName:   "",
							TargetTableName:   "",
							TargetFieldName:   "",
							OrderingFieldName: "",
						},
					},
				},
			},
			expectedErrorLen: 9,
			expectedErrors: []error{
				ast.ErrArgumentRequired,    // targetTableName
				ast.ErrArgumentRequired,    // linkedTableChecks[0].tableName
				ast.ErrArgumentRequired,    // linkedTableChecks[0]: neither link nor nav
				ast.ErrArgumentInvalidType, // linkedTableChecks[1]: both link and nav
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.targetTableName
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.targetFieldName
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.sourceTableName
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.sourceFieldName
				ast.ErrArgumentRequired,    // linkedTableChecks[2].navigationOption.orderingFieldName
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mlc := MonitoringListCheck{
				OrgId: uuid.New(),
			}

			errs := mlc.validateMonitoringListCheckConfig(tt.config)

			assert.Len(t, errs, tt.expectedErrorLen)
			for i, expectedErr := range tt.expectedErrors {
				assert.ErrorIs(t, errs[i], expectedErr)
			}
		})
	}
}

func TestMonitoringListCheck_Evaluate_Step1_ReturnsTrue(t *testing.T) {
	// Test: Step 1 returns true when target object has risk topic
	ctx := context.Background()
	orgId := uuid.New()

	// Setup mocks
	execFactory := &mocks.ExecutorFactory{}
	repo := &mockMonitoringListCheckRepository{}
	mockExec := &mocks.Executor{}

	// Mock executor factory
	execFactory.On("NewExecutor").Return(mockExec)
	execFactory.On("NewClientDbExecutor", ctx, orgId).Return(mockExec, nil)

	// Mock repository returns risk topic found
	repo.On("FindObjectRiskTopicsMetadata", ctx, mockExec, mock.MatchedBy(func(
		filter models.ObjectRiskTopicsMetadataFilter,
	) bool {
		return filter.ObjectType == utils.DummyTableNameFirst && len(filter.ObjectIds) == 1 &&
			filter.ObjectIds[0] == "target_object_123"
	})).Return([]models.ObjectMetadata{
		{Id: uuid.New(), ObjectId: "target_object_123"},
	}, nil)

	mlc := MonitoringListCheck{
		ExecutorFactory: execFactory,
		OrgId:           orgId,
		ClientObject: models.ClientObject{
			TableName: utils.DummyTableNameFirst,
			Data:      map[string]any{"object_id": "target_object_123"},
		},
		DataModel:  utils.GetDummyDataModel(),
		Repository: repo,
	}

	args := buildMonitoringListCheckArgs(ast.MonitoringListCheckConfig{
		TargetTableName: utils.DummyTableNameFirst,
		PathToTarget:    []string{},
	})

	result, errs := mlc.Evaluate(ctx, args)

	assert.Empty(t, errs)
	assert.Equal(t, true, result)
	execFactory.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestMonitoringListCheck_Evaluate_Step2_LinkToSingle_ReturnsTrue(t *testing.T) {
	// Test: Step 2 LinkToSingle returns true when linked object has risk topic
	ctx := context.Background()
	orgId := uuid.New()

	// Setup mocks
	execFactory := &mocks.ExecutorFactory{}
	repo := &mockMonitoringListCheckRepository{}
	ingestedDataReader := &mocks.IngestedDataReader{}
	mockExec := &mocks.Executor{}

	execFactory.On("NewExecutor").Return(mockExec)
	execFactory.On("NewClientDbExecutor", ctx, orgId).Return(mockExec, nil)

	// Step 1: target object has no risk topic
	repo.On("FindObjectRiskTopicsMetadata", mock.Anything, mockExec, mock.MatchedBy(func(
		filter models.ObjectRiskTopicsMetadataFilter,
	) bool {
		return filter.ObjectType == utils.DummyTableNameFirst
	})).Return([]models.ObjectMetadata{}, nil).Once()

	// Step 2: LinkToSingle - get object_id from linked table
	ingestedDataReader.On("GetDbField", mock.Anything, mockExec, mock.MatchedBy(func(params models.DbFieldReadParams) bool {
		return params.FieldName == "object_id" && len(params.Path) == 1 &&
			params.Path[0] == utils.DummyTableNameSecond
	})).Return("linked_object_456", nil)

	// Step 2: linked object has risk topic
	repo.On("FindObjectRiskTopicsMetadata", mock.Anything, mockExec, mock.MatchedBy(func(
		filter models.ObjectRiskTopicsMetadataFilter,
	) bool {
		return filter.ObjectType == utils.DummyTableNameSecond &&
			len(filter.ObjectIds) == 1 && filter.ObjectIds[0] == "linked_object_456"
	})).Return([]models.ObjectMetadata{
		{Id: uuid.New(), ObjectId: "linked_object_456"},
	}, nil)

	mlc := MonitoringListCheck{
		ExecutorFactory: execFactory,
		OrgId:           orgId,
		ClientObject: models.ClientObject{
			TableName: utils.DummyTableNameFirst,
			Data:      map[string]any{"object_id": "target_object_123"},
		},
		DataModel:          utils.GetDummyDataModel(),
		Repository:         repo,
		IngestedDataReader: ingestedDataReader,
	}

	args := buildMonitoringListCheckArgs(ast.MonitoringListCheckConfig{
		TargetTableName: utils.DummyTableNameFirst,
		PathToTarget:    []string{},
		LinkedTableChecks: []ast.LinkedTableCheck{
			{
				TableName:        utils.DummyTableNameSecond,
				LinkToSingleName: utils.Ptr(utils.DummyTableNameSecond),
			},
		},
	})

	result, errs := mlc.Evaluate(ctx, args)

	assert.Empty(t, errs)
	assert.Equal(t, true, result)
	execFactory.AssertExpectations(t)
	repo.AssertExpectations(t)
	ingestedDataReader.AssertExpectations(t)
}

func TestMonitoringListCheck_Evaluate_Step2_NavigationNotValid(t *testing.T) {
	// Test: Step 2 Navigation option is not valid (target table not found in data model)
	ctx := context.Background()
	orgId := uuid.New()

	// Setup mocks
	execFactory := &mocks.ExecutorFactory{}
	repo := &mockMonitoringListCheckRepository{}
	mockExec := &mocks.Executor{}

	execFactory.On("NewExecutor").Return(mockExec)
	execFactory.On("NewClientDbExecutor", ctx, orgId).Return(mockExec, nil)

	// Step 1: target object has no risk topic
	repo.On("FindObjectRiskTopicsMetadata", mock.Anything, mockExec, mock.MatchedBy(func(
		filter models.ObjectRiskTopicsMetadataFilter,
	) bool {
		return filter.ObjectType == utils.DummyTableNameFirst
	})).Return([]models.ObjectMetadata{}, nil).Once()

	// Data model - using a target table name that doesn't exist
	dataModel := utils.GetDummyDataModel()

	mlc := MonitoringListCheck{
		ExecutorFactory: execFactory,
		OrgId:           orgId,
		ClientObject: models.ClientObject{
			TableName: utils.DummyTableNameFirst,
			Data:      map[string]any{"object_id": "target_object_123", "source_field": "source_value"},
		},
		DataModel:  dataModel,
		Repository: repo,
	}

	args := buildMonitoringListCheckArgs(ast.MonitoringListCheckConfig{
		TargetTableName: utils.DummyTableNameFirst,
		PathToTarget:    []string{},
		LinkedTableChecks: []ast.LinkedTableCheck{
			{
				TableName: "non_existent_table",
				NavigationOption: &ast.NavigationOption{
					SourceTableName:   utils.DummyTableNameFirst,
					SourceFieldName:   "source_field",
					TargetTableName:   "non_existent_table", // This table doesn't exist in data model
					TargetFieldName:   "target_field",
					OrderingFieldName: "updated_at",
				},
			},
		},
	})

	result, errs := mlc.Evaluate(ctx, args)

	// Should return error because target table doesn't exist in data model
	assert.NotEmpty(t, errs)
	assert.Nil(t, result)
	assert.Contains(t, errs[0].Error(), "non_existent_table")
	assert.Contains(t, errs[0].Error(), "not found in data model")
}

func TestMonitoringListCheck_Evaluate_Step2_LinkToSingleFalse_FallbackNavigation_ReturnsFalse(t *testing.T) {
	// Test: LinkToSingle returns false, fallback to Navigation, returns false
	ctx := context.Background()
	orgId := uuid.New()

	// Setup mocks
	execFactory := &mocks.ExecutorFactory{}
	repo := &mockMonitoringListCheckRepository{}
	ingestedDataReader := &mocks.IngestedDataReader{}
	mockExec := &mocks.Executor{}

	execFactory.On("NewExecutor").Return(mockExec)
	execFactory.On("NewClientDbExecutor", ctx, orgId).Return(mockExec, nil)

	// Step 1: target object has no risk topic
	repo.On("FindObjectRiskTopicsMetadata", mock.Anything, mockExec, mock.MatchedBy(func(
		filter models.ObjectRiskTopicsMetadataFilter,
	) bool {
		return filter.ObjectType == utils.DummyTableNameFirst
	})).Return([]models.ObjectMetadata{}, nil).Once()

	// Step 2 LinkToSingle: get object_id from linked table
	ingestedDataReader.On("GetDbField", mock.Anything, mockExec, mock.MatchedBy(func(params models.DbFieldReadParams) bool {
		return params.FieldName == "object_id" && len(params.Path) == 1 &&
			params.Path[0] == utils.DummyTableNameSecond
	})).Return("linked_object_456", nil)

	// Step 2 LinkToSingle: linked object has no risk topic
	repo.On("FindObjectRiskTopicsMetadata", mock.Anything, mockExec, mock.MatchedBy(func(
		filter models.ObjectRiskTopicsMetadataFilter,
	) bool {
		return filter.ObjectType == utils.DummyTableNameSecond &&
			len(filter.ObjectIds) == 1 && filter.ObjectIds[0] == "linked_object_456"
	})).Return([]models.ObjectMetadata{}, nil)

	// Step 2 Navigation: list ingested objects returns empty
	ingestedDataReader.On("ListIngestedObjects", mock.Anything, mockExec, mock.Anything, mock.Anything,
		(*string)(nil), linkedTableCheckBatchSize, []string{"object_id"}).
		Return([]models.DataModelObject{}, nil)

	// Build data model with navigation option for the test
	dataModel := utils.GetDummyDataModel()
	// Add navigation option to the data model
	firstTable := dataModel.Tables[utils.DummyTableNameFirst]
	firstTable.NavigationOptions = []models.NavigationOption{
		{
			SourceTableName: utils.DummyTableNameFirst,
			SourceFieldName: utils.DummyFieldNameId,
			TargetTableName: utils.DummyTableNameThird,
			FilterFieldName: utils.DummyFieldNameId,
		},
	}
	dataModel.Tables[utils.DummyTableNameFirst] = firstTable

	mlc := MonitoringListCheck{
		ExecutorFactory: execFactory,
		OrgId:           orgId,
		ClientObject: models.ClientObject{
			TableName: utils.DummyTableNameFirst,
			Data: map[string]any{
				"object_id":            "target_object_123",
				utils.DummyFieldNameId: "source_value_789",
			},
		},
		DataModel:          dataModel,
		Repository:         repo,
		IngestedDataReader: ingestedDataReader,
	}

	args := buildMonitoringListCheckArgs(ast.MonitoringListCheckConfig{
		TargetTableName: utils.DummyTableNameFirst,
		PathToTarget:    []string{},
		LinkedTableChecks: []ast.LinkedTableCheck{
			{
				TableName:        utils.DummyTableNameSecond,
				LinkToSingleName: utils.Ptr(utils.DummyTableNameSecond),
			},
			{
				TableName: utils.DummyTableNameThird,
				NavigationOption: &ast.NavigationOption{
					SourceTableName:   utils.DummyTableNameFirst,
					SourceFieldName:   utils.DummyFieldNameId,
					TargetTableName:   utils.DummyTableNameThird,
					TargetFieldName:   utils.DummyFieldNameId,
					OrderingFieldName: "updated_at",
				},
			},
		},
	})

	result, errs := mlc.Evaluate(ctx, args)

	assert.Empty(t, errs)
	assert.Equal(t, false, result)
	execFactory.AssertExpectations(t)
	repo.AssertExpectations(t)
	ingestedDataReader.AssertExpectations(t)
}

func TestMonitoringListCheck_Evaluate_Step2_Navigation_MultipleItems_OneHasTopic_ReturnsTrue(t *testing.T) {
	// Test: Navigation returns multiple objects, one has a risk topic, returns true
	ctx := context.Background()
	orgId := uuid.New()

	// Setup mocks
	execFactory := &mocks.ExecutorFactory{}
	repo := &mockMonitoringListCheckRepository{}
	ingestedDataReader := &mocks.IngestedDataReader{}
	mockExec := &mocks.Executor{}

	execFactory.On("NewExecutor").Return(mockExec)
	execFactory.On("NewClientDbExecutor", ctx, orgId).Return(mockExec, nil)

	// Step 1: target object has no risk topic
	repo.On("FindObjectRiskTopicsMetadata", mock.Anything, mockExec, mock.MatchedBy(func(
		filter models.ObjectRiskTopicsMetadataFilter,
	) bool {
		return filter.ObjectType == utils.DummyTableNameFirst
	})).Return([]models.ObjectMetadata{}, nil).Once()

	// Step 2 Navigation: list ingested objects returns multiple items
	ingestedDataReader.On("ListIngestedObjects", mock.Anything, mockExec, mock.Anything, mock.Anything,
		(*string)(nil), linkedTableCheckBatchSize, []string{"object_id"}).
		Return([]models.DataModelObject{
			{Data: map[string]any{"object_id": "nav_object_001"}},
			{Data: map[string]any{"object_id": "nav_object_002"}},
			{Data: map[string]any{"object_id": "nav_object_003"}},
		}, nil)

	// Step 2 Navigation: one of the objects has a risk topic
	repo.On("FindObjectRiskTopicsMetadata", mock.Anything, mockExec, mock.MatchedBy(func(
		filter models.ObjectRiskTopicsMetadataFilter,
	) bool {
		return filter.ObjectType == utils.DummyTableNameThird &&
			len(filter.ObjectIds) == 3 &&
			filter.ObjectIds[0] == "nav_object_001" &&
			filter.ObjectIds[1] == "nav_object_002" &&
			filter.ObjectIds[2] == "nav_object_003"
	})).Return([]models.ObjectMetadata{
		{Id: uuid.New(), ObjectId: "nav_object_002"}, // Second object has a topic
	}, nil)

	// Build data model with navigation option for the test
	dataModel := utils.GetDummyDataModel()
	firstTable := dataModel.Tables[utils.DummyTableNameFirst]
	firstTable.NavigationOptions = []models.NavigationOption{
		{
			SourceTableName: utils.DummyTableNameFirst,
			SourceFieldName: utils.DummyFieldNameId,
			TargetTableName: utils.DummyTableNameThird,
			FilterFieldName: utils.DummyFieldNameId,
		},
	}
	dataModel.Tables[utils.DummyTableNameFirst] = firstTable

	mlc := MonitoringListCheck{
		ExecutorFactory: execFactory,
		OrgId:           orgId,
		ClientObject: models.ClientObject{
			TableName: utils.DummyTableNameFirst,
			Data: map[string]any{
				"object_id":            "target_object_123",
				utils.DummyFieldNameId: "source_value_789",
			},
		},
		DataModel:          dataModel,
		Repository:         repo,
		IngestedDataReader: ingestedDataReader,
	}

	args := buildMonitoringListCheckArgs(ast.MonitoringListCheckConfig{
		TargetTableName: utils.DummyTableNameFirst,
		PathToTarget:    []string{},
		LinkedTableChecks: []ast.LinkedTableCheck{
			{
				TableName: utils.DummyTableNameThird,
				NavigationOption: &ast.NavigationOption{
					SourceTableName:   utils.DummyTableNameFirst,
					SourceFieldName:   utils.DummyFieldNameId,
					TargetTableName:   utils.DummyTableNameThird,
					TargetFieldName:   utils.DummyFieldNameId,
					OrderingFieldName: "updated_at",
				},
			},
		},
	})

	result, errs := mlc.Evaluate(ctx, args)

	assert.Empty(t, errs)
	assert.Equal(t, true, result)
	execFactory.AssertExpectations(t)
	repo.AssertExpectations(t)
	ingestedDataReader.AssertExpectations(t)
}
