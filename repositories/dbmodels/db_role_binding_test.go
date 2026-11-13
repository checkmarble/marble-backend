package dbmodels

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptRoleBindingCustomRoleHiddenSections(t *testing.T) {
	id := uuid.New()

	custom, err := AdaptRoleBinding(DbRoleBinding{
		DbGrant:              DbGrant{Role: "org/custom", CustomRoleId: &id},
		CustomRoleFound:      true,
		CustomHiddenSections: []string{string(models.SETTINGS)},
	})
	require.NoError(t, err)
	assert.Equal(t, []models.AppSection{models.SETTINGS}, custom.HiddenSections)

	missing, err := AdaptRoleBinding(DbRoleBinding{
		DbGrant: DbGrant{Role: "org/custom", CustomRoleId: &id},
	})
	require.NoError(t, err)
	assert.Equal(t, models.AppSections(), missing.HiddenSections)
}
