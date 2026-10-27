package postgres

import (
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUserHasActiveTenantGrant(t *testing.T) {
	tenantID := uuid.New()
	query, args, err := NewQueryBuilder().
		Select("u.id").
		From("users u").
		Where(squirrel.Eq{"u.id": "user"}).
		Where(userHasActiveTenantGrant(tenantID)).
		ToSql()

	require.NoError(t, err)
	require.Contains(t, query, "EXISTS (SELECT 1 FROM active_grants g")
	require.Contains(t, query, "g.principal_id = u.id::text")
	require.Contains(t, query, "g.tenant_id = $4 OR grant_organization.tenant_id = $5")
	require.Equal(t, []any{"user", "marble", "user", tenantID.String(), tenantID.String()}, args)
}
