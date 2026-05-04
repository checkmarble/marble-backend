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

type csMetricKey struct {
	orgID uuid.UUID
	name  string
}

func TestContinuousScreeningByProviderCollector_Collect_Success(t *testing.T) {
	ctx := context.Background()
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC) // single daily period

	mockRepo := new(MockCollectorRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()

	org1Id := utils.TextToUUID("org1")
	org2Id := utils.TextToUUID("org2")
	org1PublicId := utils.TextToUUID("org1-public")
	org2PublicId := utils.TextToUUID("org2-public")
	orgs := []models.Organization{
		{Id: org1Id, Name: "Org 1", PublicId: org1PublicId},
		{Id: org2Id, Name: "Org 2", PublicId: org2PublicId},
	}
	providers := []string{"opensanctions", "ln"}

	mockRepo.On("CountCSScreeningsByProvider", ctx, mock.Anything,
		[]string{org1Id.String(), org2Id.String()}, providers, from, to).
		Return(models.ByOrgByProviderCounter{
			org1Id.String(): {"opensanctions": 20, "ln": 0},
			org2Id.String(): {"opensanctions": 0, "ln": 4},
		}, nil)

	collector := NewContinuousScreeningByProviderCollector(mockRepo, mockExecutorFactory, providers)
	metrics, err := collector.Collect(ctx, orgs, from, to)

	require.NoError(t, err)
	assert.Len(t, metrics, 4) // 2 orgs × 2 providers × 1 period

	byKey := make(map[csMetricKey]float64)
	for _, m := range metrics {
		byKey[csMetricKey{*m.PublicOrgID, m.Name}] = *m.Numeric
	}

	assert.Equal(t, float64(20), byKey[csMetricKey{org1PublicId, CSScreeningOpenSanctionsMetricName}])
	assert.Equal(t, float64(0), byKey[csMetricKey{org1PublicId, CSScreeningLNMetricName}])
	assert.Equal(t, float64(0), byKey[csMetricKey{org2PublicId, CSScreeningOpenSanctionsMetricName}])
	assert.Equal(t, float64(4), byKey[csMetricKey{org2PublicId, CSScreeningLNMetricName}])

	for _, m := range metrics {
		assert.Equal(t, from, m.From)
		assert.Equal(t, to, m.To)
	}

	mockRepo.AssertExpectations(t)
}

func TestContinuousScreeningByProviderCollector_Collect_MultiplePeriods(t *testing.T) {
	ctx := context.Background()
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC) // two daily periods

	mockRepo := new(MockCollectorRepository)
	mockExecutorFactory := executor_factory.NewExecutorFactoryStub()

	org1Id := utils.TextToUUID("org1")
	org1PublicId := utils.TextToUUID("org1-public")
	orgs := []models.Organization{
		{Id: org1Id, Name: "Org 1", PublicId: org1PublicId},
	}
	providers := []string{"opensanctions", "ln"}
	orgIds := []string{org1Id.String()}

	period1From := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	period1To := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	period2From := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	period2To := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	mockRepo.On("CountCSScreeningsByProvider", ctx, mock.Anything, orgIds, providers, period1From, period1To).
		Return(models.ByOrgByProviderCounter{
			org1Id.String(): {"opensanctions": 15, "ln": 0},
		}, nil)
	mockRepo.On("CountCSScreeningsByProvider", ctx, mock.Anything, orgIds, providers, period2From, period2To).
		Return(models.ByOrgByProviderCounter{
			org1Id.String(): {"opensanctions": 7, "ln": 3},
		}, nil)

	collector := NewContinuousScreeningByProviderCollector(mockRepo, mockExecutorFactory, providers)
	metrics, err := collector.Collect(ctx, orgs, from, to)

	require.NoError(t, err)
	assert.Len(t, metrics, 4) // 1 org × 2 providers × 2 periods

	type csPeriodicKey struct {
		name string
		from time.Time
	}
	byKey := make(map[csPeriodicKey]float64)
	for _, m := range metrics {
		byKey[csPeriodicKey{m.Name, m.From}] = *m.Numeric
	}

	assert.Equal(t, float64(15), byKey[csPeriodicKey{CSScreeningOpenSanctionsMetricName, period1From}])
	assert.Equal(t, float64(0), byKey[csPeriodicKey{CSScreeningLNMetricName, period1From}])
	assert.Equal(t, float64(7), byKey[csPeriodicKey{CSScreeningOpenSanctionsMetricName, period2From}])
	assert.Equal(t, float64(3), byKey[csPeriodicKey{CSScreeningLNMetricName, period2From}])

	mockRepo.AssertExpectations(t)
}
