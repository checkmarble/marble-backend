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
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

func TestMonitoredObjectActiveCollector_Collect_Success(t *testing.T) {
	ctx := utils.StoreLoggerInContext(context.Background(), utils.NewLogger("text"))
	to := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	yearStart := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	mockClientDbRepo := new(MockCollectorClientRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()

	org1Id := utils.TextToUUID("org1")
	org2Id := utils.TextToUUID("org2")
	org3Id := utils.TextToUUID("org3")
	org1PublicId := utils.TextToUUID("org1-public")
	org2PublicId := utils.TextToUUID("org2-public")
	org3PublicId := utils.TextToUUID("org3-public")

	orgs := []models.Organization{
		{Id: org1Id, Name: "Org with objects", PublicId: org1PublicId},
		{Id: org2Id, Name: "Org with no objects", PublicId: org2PublicId},
		{Id: org3Id, Name: "Org not set up", PublicId: org3PublicId},
	}

	// org1: set up, 50 active objects
	mockClientDbRepo.On("IsContinuousScreeningSetup", ctx, mock.Anything).Return(true, nil).Once()
	mockClientDbRepo.On("CountActiveMonitoredObjects", ctx, mock.Anything, yearStart, to).Return(50, nil).Once()

	// org2: set up, 0 active objects
	mockClientDbRepo.On("IsContinuousScreeningSetup", ctx, mock.Anything).Return(true, nil).Once()
	mockClientDbRepo.On("CountActiveMonitoredObjects", ctx, mock.Anything, yearStart, to).Return(0, nil).Once()

	// org3: not set up, skipped
	mockClientDbRepo.On("IsContinuousScreeningSetup", ctx, mock.Anything).Return(false, nil).Once()

	collector := NewMonitoredObjectActiveCollector(mockClientDbRepo, mockExecutorFactory)

	metrics, err := collector.Collect(ctx, orgs, from, to)

	require.NoError(t, err)
	assert.Len(t, metrics, 2)

	metricsByOrg := make(map[uuid.UUID]models.MetricData)
	for _, metric := range metrics {
		metricsByOrg[*metric.PublicOrgID] = metric
	}

	assert.Equal(t, float64(50), *metricsByOrg[org1PublicId].Numeric)
	assert.Equal(t, float64(0), *metricsByOrg[org2PublicId].Numeric)
	assert.Equal(t, yearStart, metricsByOrg[org1PublicId].From)
	assert.Equal(t, to, metricsByOrg[org1PublicId].To)
	_, org3Present := metricsByOrg[org3PublicId]
	assert.False(t, org3Present)

	mockClientDbRepo.AssertExpectations(t)
}

