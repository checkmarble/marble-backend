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

type screeningMetricKey struct {
	orgID uuid.UUID
	name  string
}

func TestScreeningByProviderCollector_Collect_Success(t *testing.T) {
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
	providers := []string{"opensanctions", "lexisnexis"}

	mockRepo.On("CountScreeningsByProvider", ctx, mock.Anything,
		[]string{org1Id.String(), org2Id.String()}, providers, from, to).
		Return(models.ByOrgByProviderCounter{
			org1Id.String(): {"opensanctions": 5, "lexisnexis": 0},
			org2Id.String(): {"opensanctions": 3, "lexisnexis": 1},
		}, nil)

	collector := NewScreeningByProviderCollector(mockRepo, mockExecutorFactory, providers)
	metrics, err := collector.Collect(ctx, orgs, from, to)

	require.NoError(t, err)
	assert.Len(t, metrics, 4) // 2 orgs × 2 providers × 1 period

	byKey := make(map[screeningMetricKey]float64)
	for _, m := range metrics {
		byKey[screeningMetricKey{*m.PublicOrgID, m.Name}] = *m.Numeric
	}

	assert.Equal(t, float64(5), byKey[screeningMetricKey{org1PublicId, ScreeningOpenSanctionsMetricName}])
	assert.Equal(t, float64(0), byKey[screeningMetricKey{org1PublicId, ScreeningLexisNexisMetricName}])
	assert.Equal(t, float64(3), byKey[screeningMetricKey{org2PublicId, ScreeningOpenSanctionsMetricName}])
	assert.Equal(t, float64(1), byKey[screeningMetricKey{org2PublicId, ScreeningLexisNexisMetricName}])

	for _, m := range metrics {
		assert.Equal(t, from, m.From)
		assert.Equal(t, to, m.To)
	}

	mockRepo.AssertExpectations(t)
}

func TestScreeningByProviderCollector_Collect_MultiplePeriods(t *testing.T) {
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
	providers := []string{"opensanctions", "lexisnexis"}
	orgIds := []string{org1Id.String()}

	period1From := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	period1To := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	period2From := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	period2To := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	mockRepo.On("CountScreeningsByProvider", ctx, mock.Anything, orgIds, providers, period1From, period1To).
		Return(models.ByOrgByProviderCounter{
			org1Id.String(): {"opensanctions": 10, "lexisnexis": 2},
		}, nil)
	mockRepo.On("CountScreeningsByProvider", ctx, mock.Anything, orgIds, providers, period2From, period2To).
		Return(models.ByOrgByProviderCounter{
			org1Id.String(): {"opensanctions": 8, "lexisnexis": 0},
		}, nil)

	collector := NewScreeningByProviderCollector(mockRepo, mockExecutorFactory, providers)
	metrics, err := collector.Collect(ctx, orgs, from, to)

	require.NoError(t, err)
	assert.Len(t, metrics, 4) // 1 org × 2 providers × 2 periods

	type screeningPeriodicKey struct {
		name string
		from time.Time
	}
	byKey := make(map[screeningPeriodicKey]float64)
	for _, m := range metrics {
		byKey[screeningPeriodicKey{m.Name, m.From}] = *m.Numeric
	}

	assert.Equal(t, float64(10), byKey[screeningPeriodicKey{ScreeningOpenSanctionsMetricName, period1From}])
	assert.Equal(t, float64(2), byKey[screeningPeriodicKey{ScreeningLexisNexisMetricName, period1From}])
	assert.Equal(t, float64(8), byKey[screeningPeriodicKey{ScreeningOpenSanctionsMetricName, period2From}])
	assert.Equal(t, float64(0), byKey[screeningPeriodicKey{ScreeningLexisNexisMetricName, period2From}])

	mockRepo.AssertExpectations(t)
}
