package metrics_collection

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

// Mock implementations for testing
type MockOrganizationRepository struct {
	mock.Mock
}

func (m *MockOrganizationRepository) AllOrganizations(ctx context.Context,
	exec repositories.Executor,
) ([]models.Organization, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).([]models.Organization), args.Error(1)
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

func (m *MockCollector) Collect(ctx context.Context, orgId string, from time.Time, to time.Time) ([]models.MetricData, error) {
	args := m.Called(ctx, orgId, from, to)
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

	mockOrgRepo := new(MockOrganizationRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)

	// Test organizations
	orgs := []models.Organization{
		{Id: "org1", Name: "Organization 1"},
		{Id: "org2", Name: "Organization 2"},
	}

	// Expected metrics
	globalMetrics := []models.MetricData{
		models.NewGlobalMetric("global_metric", "value1", &from, &to,
			models.MetricCollectionFrequencyMonthly),
	}

	org1Metrics := []models.MetricData{
		models.NewOrganizationMetric("org_metric", 42, "org1", &from, &to,
			models.MetricCollectionFrequencyInstant),
	}

	org2Metrics := []models.MetricData{
		models.NewOrganizationMetric("org_metric", 24, "org2", &from, &to,
			models.MetricCollectionFrequencyInstant),
	}

	// Setup expectations
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return(orgs, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return(globalMetrics, nil)
	mockOrgCollector.On("Collect", ctx, "org1", from, to).Return(org1Metrics, nil)
	mockOrgCollector.On("Collect", ctx, "org2", from, to).Return(org2Metrics, nil)

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
	}

	// Execute
	result, err := collectors.CollectMetrics(ctx, from, to)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "test-v1", result.Version)
	assert.NotEmpty(t, result.CollectionID)

	// Should have 3 metrics total (1 global + 2 org metrics)
	assert.Len(t, result.Metrics, 3)

	// Verify metrics content
	assert.True(t, slices.ContainsFunc(result.Metrics, func(m models.MetricData) bool {
		return m.OrganizationID == nil && m.Name == "global_metric"
	}), "Should contain global metric")

	assert.True(t, slices.ContainsFunc(result.Metrics, func(m models.MetricData) bool {
		return m.OrganizationID != nil && *m.OrganizationID == "org1"
	}), "Should contain org1 metric")

	assert.True(t, slices.ContainsFunc(result.Metrics, func(m models.MetricData) bool {
		return m.OrganizationID != nil && *m.OrganizationID == "org2"
	}), "Should contain org2 metric")

	// Verify all mocks were called as expected
	mockOrgRepo.AssertExpectations(t)
	mockGlobalCollector.AssertExpectations(t)
	mockOrgCollector.AssertExpectations(t)
}

func TestCollectors_CollectMetrics_GlobalCollectorError(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockOrgRepo := new(MockOrganizationRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)

	orgs := []models.Organization{
		{Id: "org1", Name: "Organization 1"},
	}

	org1Metrics := []models.MetricData{
		models.NewOrganizationMetric("org_metric", 42, "org1", &from, &to,
			models.MetricCollectionFrequencyInstant),
	}

	// Setup expectations - global collector fails, but org collector succeeds
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return(orgs, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return([]models.MetricData{}, errors.New("global collector error"))
	mockOrgCollector.On("Collect", ctx, "org1", from, to).Return(org1Metrics, nil)

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
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

	mockOrgRepo := new(MockOrganizationRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)

	globalMetrics := []models.MetricData{
		models.NewGlobalMetric("global_metric", "value1", &from, &to,
			models.MetricCollectionFrequencyMonthly),
	}

	// Setup expectations - org repo fails
	mockGlobalCollector.On("Collect", ctx, from, to).Return(globalMetrics, nil)
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return([]models.Organization{}, errors.New("database error"))

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
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

func TestCollectors_CollectMetrics_OrgCollectorError(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockOrgRepo := new(MockOrganizationRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)

	orgs := []models.Organization{
		{Id: "org1", Name: "Organization 1"},
		{Id: "org2", Name: "Organization 2"},
	}

	globalMetrics := []models.MetricData{
		models.NewGlobalMetric("global_metric", "value1", &from, &to,
			models.MetricCollectionFrequencyMonthly),
	}

	org2Metrics := []models.MetricData{
		models.NewOrganizationMetric("org_metric", 24, "org2", &from, &to,
			models.MetricCollectionFrequencyInstant),
	}

	// Setup expectations - org1 collector fails, org2 succeeds
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return(orgs, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return(globalMetrics, nil)
	mockOrgCollector.On("Collect", ctx, "org1", from, to).Return([]models.MetricData{}, errors.New("org1 collector error"))
	mockOrgCollector.On("Collect", ctx, "org2", from, to).Return(org2Metrics, nil)

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
	}

	// Execute
	result, err := collectors.CollectMetrics(ctx, from, to)

	// Assert - should succeed with global + org2 metrics (org1 skipped due to error)
	require.NoError(t, err)
	assert.Len(t, result.Metrics, 2) // 1 global + 1 org2 metric

	assert.True(t, slices.ContainsFunc(result.Metrics, func(m models.MetricData) bool {
		return m.OrganizationID == nil && m.Name == "global_metric"
	}), "Should contain global metric")

	assert.True(t, slices.ContainsFunc(result.Metrics, func(m models.MetricData) bool {
		return m.OrganizationID != nil && *m.OrganizationID == "org2"
	}), "Should contain org2 metric")

	mockOrgRepo.AssertExpectations(t)
	mockGlobalCollector.AssertExpectations(t)
	mockOrgCollector.AssertExpectations(t)
}

func TestCollectors_CollectMetrics_NoOrganizations(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockOrgRepo := new(MockOrganizationRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)

	globalMetrics := []models.MetricData{
		models.NewGlobalMetric("global_metric", "value1", &from, &to,
			models.MetricCollectionFrequencyMonthly),
	}

	// Setup expectations - no organizations
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return([]models.Organization{}, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return(globalMetrics, nil)
	// mockOrgCollector should not be called since there are no organizations

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
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

	mockOrgRepo := new(MockOrganizationRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()
	mockGlobalCollector := new(MockGlobalCollector)
	mockOrgCollector := new(MockCollector)

	orgs := []models.Organization{
		{Id: "org1", Name: "Organization 1"},
	}

	// Setup expectations - collectors return empty results
	mockOrgRepo.On("AllOrganizations", ctx, mock.Anything).Return(orgs, nil)
	mockGlobalCollector.On("Collect", ctx, from, to).Return([]models.MetricData{}, nil)
	mockOrgCollector.On("Collect", ctx, "org1", from, to).Return([]models.MetricData{}, nil)

	collectors := Collectors{
		version:                "test-v1",
		globalCollectors:       []GlobalCollector{mockGlobalCollector},
		collectors:             []Collector{mockOrgCollector},
		organizationRepository: mockOrgRepo,
		executorFactory:        mockExecutorFactory,
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

func TestNewCollectorsTestV1(t *testing.T) {
	// Setup
	mockOrgRepo := new(MockOrganizationRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()

	// Execute
	collectors := NewCollectorsTestV1(mockExecutorFactory, mockOrgRepo)

	// Assert
	assert.Equal(t, "test-v1", collectors.version)
	assert.Len(t, collectors.globalCollectors, 1)
	assert.Len(t, collectors.collectors, 1)
	assert.Equal(t, mockOrgRepo, collectors.organizationRepository)
	assert.Equal(t, mockExecutorFactory, collectors.executorFactory)

	// Verify the collectors are of the expected stub types
	_, isStubGlobal := collectors.globalCollectors[0].(StubGlobalCollector)
	assert.True(t, isStubGlobal, "Should contain StubGlobalCollector")

	_, isStubOrg := collectors.collectors[0].(StubOrganizationCollector)
	assert.True(t, isStubOrg, "Should contain StubOrganizationCollector")
}
