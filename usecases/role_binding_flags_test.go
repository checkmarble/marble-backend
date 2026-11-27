package usecases

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

func TestCheckRoleBindingFeatureFlagsCustomRoles(t *testing.T) {
	orgId := uuid.New()
	native := models.RoleBinding{Role: models.ADMIN}
	custom := models.RoleBinding{Role: models.Role("org/custom.name")}

	t.Run("disabled", func(t *testing.T) {
		t.Setenv("ENABLE_CUSTOM_ROLES", "")

		assert.NoError(t, checkRoleBindingFeatureFlags(orgId, native))
		assert.ErrorIs(t, checkRoleBindingFeatureFlags(orgId, custom), models.BadParameterError)
	})

	t.Run("enabled for another organization", func(t *testing.T) {
		t.Setenv("ENABLE_CUSTOM_ROLES", uuid.NewString())

		assert.ErrorIs(t, checkRoleBindingFeatureFlags(orgId, custom), models.BadParameterError)
	})

	t.Run("enabled for organization", func(t *testing.T) {
		t.Setenv("ENABLE_CUSTOM_ROLES", orgId.String())

		assert.NoError(t, checkRoleBindingFeatureFlags(orgId, native))
		assert.NoError(t, checkRoleBindingFeatureFlags(orgId, custom))
	})
}
