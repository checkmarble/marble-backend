package dbmodels

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptRoleBindingCustomRoleHiddenSections(t *testing.T) {
	grant := DbGrant{Role: "org/custom", CustomRoleId: new(uuid.New()), Conditions: json.RawMessage(`{}`)}

	custom, err := AdaptRoleBinding(DbRoleBinding{
		DbGrant:              grant,
		CustomRoleFound:      true,
		CustomHiddenSections: []string{string(models.SETTINGS)},
	})
	require.NoError(t, err)
	assert.Equal(t, []models.AppSection{models.SETTINGS}, custom.HiddenSections)

	missing, err := AdaptRoleBinding(DbRoleBinding{DbGrant: grant})
	require.NoError(t, err)
	assert.Equal(t, models.AppSections(), missing.HiddenSections)
}
