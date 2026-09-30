package dbmodels

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdaptTenant(t *testing.T) {
	id := uuid.New()
	got, err := AdaptTenant(DBTenant{Id: id, Name: "Tenant"})
	require.NoError(t, err)
	require.Equal(t, models.Tenant{Id: id, Name: "Tenant"}, got)
}
