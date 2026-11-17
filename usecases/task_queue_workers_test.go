package usecases

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/require"
)

func TestOrganizationQueueWorkers(t *testing.T) {
	config := OrganizationQueueWorkers{
		models.OrganizationEnvironmentProduction: 9,
		models.OrganizationEnvironmentDemo:       8,
		models.OrganizationEnvironmentStaging:    7,
		models.OrganizationEnvironmentUnknown:    6,
	}
	orgs := []models.Organization{
		{Id: uuid.New(), Environment: models.OrganizationEnvironmentProduction},
		{Id: uuid.New(), Environment: models.OrganizationEnvironmentDemo},
		{Id: uuid.New(), Environment: models.OrganizationEnvironmentDemo},
		{Id: uuid.New(), Environment: models.OrganizationEnvironmentStaging},
		{Id: uuid.New(), Environment: models.OrganizationEnvironmentUnknown},
		{Id: uuid.New(), Environment: models.OrganizationEnvironment(999)},
	}
	queues := config.queuesForOrganizations(orgs)
	require.Len(t, queues, 6)
	for i, expected := range []int{9, 8, 8, 7, 6, 6} {
		require.Equal(t, expected, queues[orgs[i].Id.String()].MaxWorkers)
	}
}

func TestGlobalQueueWorkersUnchanged(t *testing.T) {
	for _, queues := range []map[string]river.QueueConfig{
		QueueMetrics(),
		QueueBilling(),
		QueueAnalyticsMerge(),
		QueueContinuousScreeningDatasetUpdate(),
		QueueWebhookCleanup(),
		QueueAsyncDecisionCleanup(),
	} {
		require.Len(t, queues, 1)
		for _, queue := range queues {
			require.Equal(t, 1, queue.MaxWorkers)
		}
	}
}
