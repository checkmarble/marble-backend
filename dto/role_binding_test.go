package dto

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoleBindingUsesSlugAsRoleReference(t *testing.T) {
	binding := AdaptRoleBinding(models.RoleBinding{Role: models.ADMIN})

	payload, err := json.Marshal(binding)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, "ADMIN", decoded["role"])
	assert.IsType(t, "", decoded["role"])
}
