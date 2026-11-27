package usecases

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
)

func TestCheckRoleBindingFeatureFlagsCaveats(t *testing.T) {
	orgId := uuid.New()
	notAfter := time.Now().Add(time.Hour)
	unconditional := models.RoleBinding{Role: models.ADMIN}
	conditional := models.RoleBinding{
		Role:       models.ADMIN,
		Conditions: models.RoleBindingConditions{NotAfter: &notAfter},
	}

	t.Run("disabled", func(t *testing.T) {
		t.Setenv("ENABLE_ROLE_BINDING_CAVEATS", "")

		assert.NoError(t, checkRoleBindingFeatureFlags(orgId, unconditional))
		assert.ErrorIs(t, checkRoleBindingFeatureFlags(orgId, conditional), models.BadParameterError)
	})

	t.Run("enabled for another organization", func(t *testing.T) {
		t.Setenv("ENABLE_ROLE_BINDING_CAVEATS", uuid.NewString())

		assert.ErrorIs(t, checkRoleBindingFeatureFlags(orgId, conditional), models.BadParameterError)
	})

	t.Run("enabled for organization", func(t *testing.T) {
		t.Setenv("ENABLE_ROLE_BINDING_CAVEATS", orgId.String())

		assert.NoError(t, checkRoleBindingFeatureFlags(orgId, unconditional))
		assert.NoError(t, checkRoleBindingFeatureFlags(orgId, conditional))
	})
}

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
