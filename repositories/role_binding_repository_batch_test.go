package repositories

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var roleBindingRowColumns = []string{
	"id", "principal_type", "principal_id", "tenant_id", "organization_id",
	"role", "custom_role_id", "conditions", "custom_permissions",
}

func roleBindingRow(principalType, principalId string, orgId *uuid.UUID, role models.Role) []any {
	return []any{
		uuid.New(), principalType, principalId, (*uuid.UUID)(nil), orgId,
		string(role), (*uuid.UUID)(nil), json.RawMessage(`{}`), []string{},
	}
}

func newMarbleQueryExecutor(t *testing.T) *graphQueryExecutor {
	exec := newGraphQueryExecutor(t)
	exec.schemaType = models.DATABASE_SCHEMA_TYPE_MARBLE
	return exec
}

func TestListUsersLoadsRoleBindingsInOneQuery(t *testing.T) {
	ctx := context.Background()
	exec := newMarbleQueryExecutor(t)
	orgId := uuid.New()
	alice, bob, carol := uuid.NewString(), uuid.NewString(), uuid.NewString()

	userRow := func(id string) []any {
		return []any{id, id + "@example.com", 0, &orgId, pgtype.Text{}, pgtype.Text{},
			pgtype.Timestamptz{}, false, ""}
	}
	exec.pool.ExpectQuery(".*").WithArgs(pgxmock.AnyArg()).WillReturnRows(
		pgxmock.NewRows([]string{
			"id", "email", "role", "organization_id", "first_name", "last_name",
			"deleted_at", "ai_assist_enabled", "picture",
		}).AddRow(userRow(alice)...).AddRow(userRow(bob)...).AddRow(userRow(carol)...))
	// Every user is queried at once, scoped by its own organization.
	exec.pool.ExpectQuery(".*").
		WithArgs("marble", alice, bob, carol, "user").
		WillReturnRows(pgxmock.NewRows(roleBindingRowColumns).
			AddRow(roleBindingRow("user", alice, &orgId, models.ADMIN)...).
			AddRow(roleBindingRow("user", bob, &orgId, models.VIEWER)...).
			AddRow(roleBindingRow("user", alice, nil, models.MARBLE_ADMIN)...))

	users, err := (&MarbleDbRepository{}).ListUsers(ctx, exec, &orgId)

	require.NoError(t, err)
	require.NoError(t, exec.pool.ExpectationsWereMet())
	require.Len(t, exec.queries, 2, "users and all their role bindings are loaded in two queries")
	assert.Contains(t, exec.queries[1], "active_grants")
	assert.Contains(t, exec.queries[1], "JOIN users u")

	require.Len(t, users, 3)
	assert.Equal(t, []models.Role{models.ADMIN, models.MARBLE_ADMIN}, models.RoleNames(users[0].RoleBindings))
	assert.Equal(t, []models.Role{models.VIEWER}, models.RoleNames(users[1].RoleBindings))
	assert.Empty(t, users[2].RoleBindings)
}

func TestListApiKeysLoadsRoleBindingsInOneQuery(t *testing.T) {
	ctx := context.Background()
	exec := newMarbleQueryExecutor(t)
	orgId := uuid.New()
	first, second := uuid.New(), uuid.New()

	apiKeyRow := func(id uuid.UUID) []any {
		return []any{id, time.Now(), pgtype.Timestamptz{}, "key", []byte("hash"), "abc", orgId, 0}
	}
	exec.pool.ExpectQuery(".*").WithArgs(pgxmock.AnyArg()).WillReturnRows(
		pgxmock.NewRows([]string{
			"id", "created_at", "deleted_at", "description", "key_hash", "prefix", "org_id", "role",
		}).AddRow(apiKeyRow(first)...).AddRow(apiKeyRow(second)...))
	// Every API key is queried at once, scoped to the organization.
	exec.pool.ExpectQuery(".*").
		WithArgs("marble", first.String(), second.String(), "api_key", orgId.String()).
		WillReturnRows(pgxmock.NewRows(roleBindingRowColumns).
			AddRow(roleBindingRow("api_key", second.String(), &orgId, models.API_CLIENT)...))

	apiKeys, err := (&MarbleDbRepository{}).ListApiKeys(ctx, exec, orgId)

	require.NoError(t, err)
	require.NoError(t, exec.pool.ExpectationsWereMet())
	require.Len(t, exec.queries, 2, "API keys and all their role bindings are loaded in two queries")
	assert.True(t, strings.Contains(exec.queries[1], "active_grants"))

	require.Len(t, apiKeys, 2)
	assert.Empty(t, apiKeys[0].RoleBindings)
	assert.Equal(t, []models.Role{models.API_CLIENT}, models.RoleNames(apiKeys[1].RoleBindings))
}
