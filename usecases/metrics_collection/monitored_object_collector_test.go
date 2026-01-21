package metrics_collection

import (
	"context"
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

// Test mocks for ContinuousScreeningCollector
type MockContinuousScreeningMarbleDbRepository struct {
	mock.Mock
}

func (m *MockContinuousScreeningMarbleDbRepository) GetEnabledConfigStableIdsByOrg(
	ctx context.Context,
	exec repositories.Executor,
	orgIds []string,
) (map[string][]uuid.UUID, error) {
	args := m.Called(ctx, exec, orgIds)
	return args.Get(0).(map[string][]uuid.UUID), args.Error(1)
}

type MockContinuousScreeningClientDbRepository struct {
	mock.Mock
}

func (m *MockContinuousScreeningClientDbRepository) CountMonitoredObjectsByConfigStableIds(
	ctx context.Context,
	exec repositories.Executor,
	configStableIds []uuid.UUID,
) (int, error) {
	args := m.Called(ctx, exec, configStableIds)
	return args.Get(0).(int), args.Error(1)
}

// TestContinuousScreeningCollector_Collect_Success tests the successful collection with multiple scenarios:
// - Org with monitored objects
// - Org with enabled configs but no monitored objects
// - Org with no enabled configs
func TestContinuousScreeningCollector_Collect_Success(t *testing.T) {
	// Setup
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	mockMarbleDbRepo := new(MockContinuousScreeningMarbleDbRepository)
	mockClientDbRepo := new(MockContinuousScreeningClientDbRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()

	// Test organizations
	org1Id := utils.TextToUUID("org1")
	org2Id := utils.TextToUUID("org2")
	org3Id := utils.TextToUUID("org3")
	org1PublicId := utils.TextToUUID("org1-public")
	org2PublicId := utils.TextToUUID("org2-public")
	org3PublicId := utils.TextToUUID("org3-public")
	orgs := []models.Organization{
		{Id: org1Id, Name: "Org with objects", PublicId: org1PublicId},
		{Id: org2Id, Name: "Org with no objects", PublicId: org2PublicId},
		{Id: org3Id, Name: "Org with no configs", PublicId: org3PublicId},
	}

	// Mock config stable IDs
	config1 := uuid.New()
	config2 := uuid.New()
	configStableIdsByOrg := map[string][]uuid.UUID{
		org1Id.String(): {config1},
		org2Id.String(): {config2},
		org3Id.String(): {},
	}

	// Setup expectations
	mockMarbleDbRepo.On("GetEnabledConfigStableIdsByOrg", ctx, mock.Anything, []string{
		org1Id.String(), org2Id.String(), org3Id.String(),
	}).Return(configStableIdsByOrg, nil)

	mockClientDbRepo.On("CountMonitoredObjectsByConfigStableIds", ctx, mock.Anything,
		[]uuid.UUID{config1}).Return(100, nil)
	mockClientDbRepo.On("CountMonitoredObjectsByConfigStableIds", ctx, mock.Anything,
		[]uuid.UUID{config2}).Return(0, nil)
	mockClientDbRepo.On("CountMonitoredObjectsByConfigStableIds", ctx, mock.Anything,
		[]uuid.UUID{}).Return(0, nil)

	// Create collector
	collector := NewContinuousScreeningCollector(
		mockMarbleDbRepo,
		mockClientDbRepo,
		mockExecutorFactory,
	)

	// Execute
	metrics, err := collector.Collect(ctx, orgs, from, to)

	// Assert
	require.NoError(t, err)
	assert.Len(t, metrics, 3)

	// Verify metrics by organization
	metricsByOrg := make(map[uuid.UUID]models.MetricData)
	for _, metric := range metrics {
		metricsByOrg[*metric.PublicOrgID] = metric
	}

	assert.Equal(t, float64(100), *metricsByOrg[org1PublicId].Numeric)
	assert.Equal(t, float64(0), *metricsByOrg[org2PublicId].Numeric)
	assert.Equal(t, float64(0), *metricsByOrg[org3PublicId].Numeric)

	mockMarbleDbRepo.AssertExpectations(t)
	mockClientDbRepo.AssertExpectations(t)
}
