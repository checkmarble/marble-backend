package dto

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
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

func TestCustomRoleBindingSlugRoundTrip(t *testing.T) {
	slug := models.Role("org/custom.name")
	binding := RoleBinding{Role: slug}

	adapted := AdaptRoleBindingInput(binding)
	assert.Equal(t, slug, adapted.Role)
	assert.Nil(t, adapted.CustomRoleId)
	assert.Equal(t, slug, AdaptRoleBinding(adapted).Role)
}

func TestRoleDTOOmitsInternalIdentifier(t *testing.T) {
	role := AdaptRole(models.RbacRole{
		Id:          uuid.New(),
		Slug:        "org/custom.name",
		Name:        "Custom name",
		Permissions: []models.Permission{models.CASE_READ_WRITE},
	})

	payload, err := json.Marshal(role)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.NotContains(t, decoded, "id")
	assert.Equal(t, "org/custom.name", decoded["slug"])
	assert.Equal(t, "Custom name", decoded["name"])
}
