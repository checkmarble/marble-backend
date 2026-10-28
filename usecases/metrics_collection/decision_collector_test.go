package metrics_collection

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

func TestDecisionCollector_Collect_SingleCallPerPeriod(t *testing.T) {
	org := models.Organization{Id: utils.TextToUUID("org1"), Name: "Org 1", PublicId: utils.TextToUUID("org1")}
	orgIds := []string{org.Id.String()}

	tests := []struct {
		name string
		from time.Time
		to   time.Time
	}{
		{
			name: "day-aligned period",
			from: time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "partial day",
			from: time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := new(MockCollectorRepository)
			repo.On("CountDecisionsByOrg", ctx, mock.Anything, orgIds, tt.from, tt.to).
				Return(map[string]int{org.Id.String(): 5}, nil).Once()

			collector := NewDecisionCollector(repo, executor_factory.NewExecutorFactoryStub())

			metrics, err := collector.Collect(ctx, []models.Organization{org}, tt.from, tt.to)

			require.NoError(t, err)
			repo.AssertNumberOfCalls(t, "CountDecisionsByOrg", 1)
			repo.AssertExpectations(t)
			require.Len(t, metrics, 1)
			assert.Equal(t, float64(5), *metrics[0].Numeric)
		})
	}
}
