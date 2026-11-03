package repositories

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

func TestRoleBindingScopes(t *testing.T) {
	orgId := uuid.New()

	t.Run("home scope includes platform bindings", func(t *testing.T) {
		sql, args, err := homeScope(orgId).where("g.").ToSql()
		require.NoError(t, err)
		assert.Equal(t, "(g.organization_id = ? OR g.organization_id IS NULL AND g.tenant_id IS NULL)", sql)
		assert.Equal(t, []any{orgId.String()}, args)
	})

	t.Run("organization scope only includes the organization's bindings", func(t *testing.T) {
		sql, args, err := organizationScope(orgId).where("g.").ToSql()
		require.NoError(t, err)
		assert.Equal(t, "g.organization_id = ?", sql)
		assert.Equal(t, []any{orgId.String()}, args)
	})
}

func TestReplaceUserOrganizationRoleBindingsRefusesPlatformRoles(t *testing.T) {
	repo := &MarbleDbRepository{}

	err := repo.ReplaceUserOrganizationRoleBindings(context.Background(), nil, uuid.New(), "user",
		[]models.RoleBinding{models.NewNativeRoleBinding(models.MARBLE_ADMIN)})
	assert.ErrorIs(t, err, models.BadParameterError)

	err = repo.ReplaceUserOrganizationRoleBindings(context.Background(), nil, uuid.Nil, "user",
		[]models.RoleBinding{models.NewNativeRoleBinding(models.VIEWER)})
	assert.ErrorIs(t, err, models.BadParameterError)
}
