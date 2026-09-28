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

func TestRoleBindingTimeOfDayRoundTrip(t *testing.T) {
	var input RoleBinding
	require.NoError(t, json.Unmarshal([]byte(`{"role":"VIEWER","conditions":{"time_of_day":[2200,600]}}`), &input))

	binding := AdaptRoleBindingInput(input)
	require.NotNil(t, binding.Conditions.TimeOfDay)
	assert.Equal(t, models.TimeOfDayRange{2200, 600}, *binding.Conditions.TimeOfDay)

	payload, err := json.Marshal(AdaptRoleBinding(binding).Conditions)
	require.NoError(t, err)
	assert.JSONEq(t, `{"time_of_day":[2200,600]}`, string(payload))

	assert.Error(t, json.Unmarshal([]byte(`{"role":"VIEWER","conditions":{"time_of_day":[900,1960]}}`), &input))
}
