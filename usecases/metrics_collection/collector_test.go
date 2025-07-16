package metrics_collection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

// Mock implementations for testing
type MockCollectorRepository struct {
	mock.Mock
}

type MockDecisionCollectorRepository struct {
	mock.Mock
}
type MockCaseCollectorRepository struct {
	mock.Mock
}
type MockMetadataRepository struct {
	mock.Mock
}

func (m *MockCollectorRepository) AllOrganizations(ctx context.Context,
	exec repositories.Executor,
) ([]models.Organization, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).([]models.Organization), args.Error(1)
}

func (m *MockDecisionCollectorRepository) CountDecisionsByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
	from, to time.Time,
) (map[string]int, error) {
	args := m.Called(ctx, exec, orgIds, from, to)
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockCaseCollectorRepository) CountCasesByOrg(ctx context.Context, exec repositories.Executor, orgIds []string,
	from, to time.Time,
) (map[string]int, error) {
	args := m.Called(ctx, exec, orgIds, from, to)
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockMetadataRepository) GetMetadata(ctx context.Context, exec repositories.Executor, orgID *uuid.UUID,
	key models.MetadataKey,
) (models.Metadata, error) {
	args := m.Called(ctx, exec, orgID, key)
	return args.Get(0).(models.Metadata), args.Error(1)
}

type MockGlobalCollector struct {
	mock.Mock
}

func (m *MockGlobalCollector) Collect(ctx context.Context, from time.Time, to time.Time) ([]models.MetricData, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).([]models.MetricData), args.Error(1)
}

type MockCollector struct {
	mock.Mock
}

func (m *MockCollector) Collect(ctx context.Context, orgIds []string, from time.Time, to time.Time) ([]models.MetricData, error) {
	args := m.Called(ctx, orgIds, from, to)
	return args.Get(0).([]models.MetricData), args.Error(1)
}

// This test verifies the successful collection of metrics from both global and organization-specific collectors.
// The test setup includes:
// - Mock organization repository that returns 2 test organizations
// - Mock global collector that returns 1 global metric
// - Mock organization collector that returns different metrics for each organization
// - Test time range from Jan 1 to Jan 31, 2024
//
// The test validates that:
// 1. All organizations are fetched from the repository
// 2. Global metrics are collected once for the entire time range
// 3. Organization-specific metrics are collected for each organization
// 4. All metrics are combined into a single MetricsCollection result
// 5. The result contains the expected CollectionID and Version
// 6. No errors occur during the collection process
func TestCollectors_CollectMetrics_Success(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockOrgRepo := new(MockCollectorRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)
	mockMetadataRepo := new(MockMetadataRepository)

	// Test organizations
	orgs := []models.Organization{
		{Id: "org1", Name: "Organization 1"},
		{Id: "org2", Name: "Organization 2"},
	}

	// Expected metrics
	globalMetrics := []models.MetricData{
		models.NewGlobalMetric("global_metric", nil, utils.Ptr("value1"), from, to),
	}

	org1Metrics := []models.MetricData{
		models.NewOrganizationMetric("org_metric", utils.Ptr(float64(42)), nil, "org1", from, to),
	}

	org2Metrics := []models.MetricData{
		models.NewOrganizationMetric("org_metric", utils.Ptr(float64(24)), nil, "org2", from, to),
	}

	// Setup expectations
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return(orgs, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return(globalMetrics, nil)
	mockOrgCollector.On("Collect", ctx, []string{"org1", "org2"}, from, to).Return([]models.MetricData{
		org1Metrics[0], org2Metrics[0],
	}, nil)
	deploymentID := uuid.New()
	mockMetadataRepo.On("GetMetadata", ctx, mock.Anything, (*uuid.UUID)(nil),
		models.MetadataKeyDeploymentID).Return(models.Metadata{
		Value: deploymentID.String(),
	}, nil)
	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
		metadataRepository:     mockMetadataRepo,
	}

	// Execute
	result, err := collectors.CollectMetrics(ctx, from, to)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "test-v1", result.Version)
	assert.NotEmpty(t, result.CollectionID)
	assert.Equal(t, deploymentID, result.DeploymentID)
	// Should have 3 metrics total (1 global + 2 org metrics)
	assert.Len(t, result.Metrics, 3)

	// Verify metrics content
	assert.Contains(t, result.Metrics, globalMetrics[0], "Should contain global metric")
	assert.Contains(t, result.Metrics, org1Metrics[0], "Should contain org1 metric")
	assert.Contains(t, result.Metrics, org2Metrics[0], "Should contain org2 metric")

	// Verify all mocks were called as expected
	mockOrgRepo.AssertExpectations(t)
	mockGlobalCollector.AssertExpectations(t)
	mockOrgCollector.AssertExpectations(t)
	mockMetadataRepo.AssertExpectations(t)
}

func TestCollectors_CollectMetrics_GlobalCollectorError(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockOrgRepo := new(MockCollectorRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)
	mockMetadataRepo := new(MockMetadataRepository)
	orgs := []models.Organization{
		{Id: "org1", Name: "Organization 1"},
	}

	org1Metrics := []models.MetricData{
		models.NewOrganizationMetric("org_metric", utils.Ptr(float64(42)), nil, "org1", from, to),
	}

	// Setup expectations - global collector fails, but org collector succeeds
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return(orgs, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return([]models.MetricData{}, errors.New("global collector error"))
	mockOrgCollector.On("Collect", ctx, []string{"org1"}, from, to).Return([]models.MetricData{
		org1Metrics[0],
	}, nil)
	mockMetadataRepo.On("GetMetadata", ctx, mock.Anything, (*uuid.UUID)(nil),
		models.MetadataKeyDeploymentID).Return(models.Metadata{
		Value: uuid.New().String(),
	}, nil)

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
		metadataRepository:     mockMetadataRepo,
	}

	// Execute
	result, err := collectors.CollectMetrics(ctx, from, to)

	// Assert - should succeed but only have org metrics
	require.NoError(t, err)
	assert.Len(t, result.Metrics, 1)
	assert.Equal(t, "org1", *result.Metrics[0].OrganizationID)

	mockOrgRepo.AssertExpectations(t)
	mockGlobalCollector.AssertExpectations(t)
	mockOrgCollector.AssertExpectations(t)
}

func TestCollectors_CollectMetrics_OrganizationRepositoryError(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockOrgRepo := new(MockCollectorRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockMetadataRepo := new(MockMetadataRepository)
	globalMetrics := []models.MetricData{
		models.NewGlobalMetric("global_metric", nil, utils.Ptr("value1"), from, to),
	}

	// Setup expectations - org repo fails
	mockGlobalCollector.On("Collect", ctx, from, to).Return(globalMetrics, nil)
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return([]models.Organization{}, errors.New("database error"))
	mockMetadataRepo.On("GetMetadata", ctx, mock.Anything, (*uuid.UUID)(nil),
		models.MetadataKeyDeploymentID).Return(models.Metadata{
		Value: uuid.New().String(),
	}, nil)

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
		metadataRepository:     mockMetadataRepo,
	}

	// Execute
	result, err := collectors.CollectMetrics(ctx, from, to)

	// Assert - should fail due to org repo error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
	assert.Empty(t, result.Metrics)

	mockOrgRepo.AssertExpectations(t)
	mockGlobalCollector.AssertExpectations(t)
}

func TestCollectors_CollectMetrics_NoOrganizations(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockOrgRepo := new(MockCollectorRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)
	mockMetadataRepo := new(MockMetadataRepository)
	globalMetrics := []models.MetricData{
		models.NewGlobalMetric("global_metric", nil, utils.Ptr("value1"), from, to),
	}

	// Setup expectations - no organizations
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return([]models.Organization{}, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return(globalMetrics, nil)
	mockOrgCollector.On("Collect", ctx, []string{}, from, to).Return([]models.MetricData{}, nil)
	// mockOrgCollector should not be called since there are no organizations

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
		metadataRepository:     mockMetadataRepo,
	}

	// Execute
	result, err := collectors.CollectMetrics(ctx, from, to)

	// Assert - should only have global metrics
	require.NoError(t, err)
	assert.Len(t, result.Metrics, 1)
	assert.Nil(t, result.Metrics[0].OrganizationID)

	mockOrgRepo.AssertExpectations(t)
	mockGlobalCollector.AssertExpectations(t)
	// mockOrgCollector should not have been called
}

func TestCollectors_CollectMetrics_EmptyResults(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockOrgRepo := new(MockCollectorRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)
	mockMetadataRepo := new(MockMetadataRepository)
	orgs := []models.Organization{
		{Id: "org1", Name: "Organization 1"},
	}

	// Setup expectations - collectors return empty results
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return(orgs, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return([]models.MetricData{}, nil)
	mockOrgCollector.On("Collect", ctx, []string{"org1"}, from, to).Return([]models.MetricData{}, nil)
	mockMetadataRepo.On("GetMetadata", ctx, mock.Anything, (*uuid.UUID)(nil),
		models.MetadataKeyDeploymentID).Return(models.Metadata{
		Value: uuid.New().String(),
	}, nil)

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
		metadataRepository:     mockMetadataRepo,
	}

	// Execute
	result, err := collectors.CollectMetrics(ctx, from, to)

	// Assert - should succeed with empty metrics
	require.NoError(t, err)
	assert.Empty(t, result.Metrics)
	assert.Equal(t, "test-v1", result.Version)
	assert.NotEmpty(t, result.CollectionID)

	mockOrgRepo.AssertExpectations(t)
	mockGlobalCollector.AssertExpectations(t)
	mockOrgCollector.AssertExpectations(t)
}

func TestNewCollectorsV1(t *testing.T) {
	// Setup
	mockOrgRepo := new(MockCollectorRepository)
	mockDecisionRepo := new(MockDecisionCollectorRepository)
	mockCaseRepo := new(MockCaseCollectorRepository)
	mockMetadataRepo := new(MockMetadataRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()

	// Execute
	collectors := NewCollectorsV1(mockExecutorFactory, mockOrgRepo, mockDecisionRepo,
		mockCaseRepo, mockMetadataRepo, "ApiVersionTest", models.LicenseConfiguration{})

	// Assert
	assert.Equal(t, "v1", collectors.version)
	assert.Len(t, collectors.globalCollectors, 1)
	assert.Len(t, collectors.collectors, 2)
	assert.Equal(t, mockOrgRepo, collectors.organizationRepository)
	assert.Equal(t, mockExecutorFactory, collectors.executorFactory)

	// Verify the collectors are of the expected stub types
	_, isAppVersionCollector := collectors.globalCollectors[0].(AppVersionCollector)
	assert.True(t, isAppVersionCollector, "Should contain AppVersionCollector")

	_, isDecisionCollector := collectors.collectors[0].(DecisionCollector)
	assert.True(t, isDecisionCollector, "Should contain DecisionCollector")
}
