package cmd

import (
	"os"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/stretchr/testify/require"
)

func TestLoadOrganizationQueueWorkers(t *testing.T) {
	names := []string{
		"MAX_WORKERS_PRODUCTION_ORG",
		"MAX_WORKERS_DEMO_ORG",
		"MAX_WORKERS_STAGING_ORG",
		"MAX_WORKERS_UNKNOWN_ORG",
	}
	for _, name := range names {
		t.Setenv(name, "1")
	}
	// Clear the variables rather than setting them empty: empty is invalid input.
	for _, name := range names {
		require.NoError(t, os.Unsetenv(name))
	}
	got, err := loadOrganizationQueueWorkers()
	require.NoError(t, err)
	require.Equal(t, usecases.OrganizationQueueWorkers{
		models.OrganizationEnvironmentProduction: 5,
		models.OrganizationEnvironmentDemo:       2,
		models.OrganizationEnvironmentStaging:    2,
		models.OrganizationEnvironmentUnknown:    2,
	}, got)

	for i, name := range names {
		t.Setenv(name, []string{"9", "8", "7", "6"}[i])
	}
	got, err = loadOrganizationQueueWorkers()
	require.NoError(t, err)
	require.Equal(t, usecases.OrganizationQueueWorkers{
		models.OrganizationEnvironmentProduction: 9,
		models.OrganizationEnvironmentDemo:       8,
		models.OrganizationEnvironmentStaging:    7,
		models.OrganizationEnvironmentUnknown:    6,
	}, got)

	for _, name := range names {
		for _, value := range []string{"", "bad", "0", "-1"} {
			t.Run(name+"/"+value, func(t *testing.T) {
				t.Setenv(name, value)
				_, err := loadOrganizationQueueWorkers()
				require.ErrorContains(t, err, name)
			})
		}
	}
}
