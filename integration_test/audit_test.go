package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUserAndAPIKeyAudit(t *testing.T) {
	const actorUser = "audit-test-actor"
	actorKey, scopeOrg, scopeTenant := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, table := range []string{"users", "api_keys"} {
		for _, tc := range []struct {
			name, user, key, org, tenant string
			wantEvents                   int
		}{
			{"user organization", actorUser, "", scopeOrg, scopeTenant, 1},
			{"API key organization", "", actorKey, scopeOrg, scopeTenant, 1},
			{"both actors", actorUser, actorKey, scopeOrg, scopeTenant, 1},
			{"tenant only", actorUser, "", "", scopeTenant, 1},
			{"platform", actorUser, "", "", "", 1},
			{"no actor", "", "", scopeOrg, scopeTenant, 0},
		} {
			t.Run(table+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				tx, err := pgPool.Begin(ctx)
				require.NoError(t, err)
				defer tx.Rollback(ctx)
				_, err = tx.Exec(ctx, `
INSERT INTO tenants (id, name) VALUES ('22222222-2222-2222-2222-222222222222', 'audit-test');
INSERT INTO organizations (id, name, tenant_id) VALUES
('11111111-1111-1111-1111-111111111111', 'audit-test', '22222222-2222-2222-2222-222222222222');`)
				require.NoError(t, err)
				_, err = tx.Exec(ctx, `SELECT
					set_config('custom.current_user_id', $1, true),
					set_config('custom.current_api_key_id', $2, true),
					set_config('custom.current_org_id', $3, true),
					set_config('custom.current_tenant_id', $4, true)`,
					tc.user, tc.key, tc.org, tc.tenant)
				require.NoError(t, err)

				id := uuid.New()
				queries := []string{
					`INSERT INTO users (id, email, role) VALUES ($1, 'audit-test@example.com', 1) RETURNING to_jsonb(users)`,
					`UPDATE users SET role = 2 WHERE id = $1 RETURNING to_jsonb(users)`,
					`DELETE FROM users WHERE id = $1 RETURNING to_jsonb(users)`,
				}
				if table == "api_keys" {
					queries = []string{
						`INSERT INTO api_keys (id, org_id, prefix, role, description, key_hash)
						 VALUES ($1, '11111111-1111-1111-1111-111111111111', 'audit-test', 1, 'original', decode('010203', 'hex'))
						 RETURNING to_jsonb(api_keys)`,
						`UPDATE api_keys SET description = 'updated', deleted_at = now() WHERE id = $1 RETURNING to_jsonb(api_keys)`,
						`DELETE FROM api_keys WHERE id = $1 RETURNING to_jsonb(api_keys)`,
					}
				}
				var previous string
				for i, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
					var current string
					require.NoError(t, tx.QueryRow(ctx, queries[i], id).Scan(&current))
					var count int
					require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM audit.audit_events
						WHERE "table" = $1 AND entity_id = $2 AND operation = $3`, table, id, operation).Scan(&count))
					require.Equal(t, tc.wantEvents, count, operation)
					if count == 0 {
						continue
					}

					var data string
					var oldData *string
					var user, key, org, tenant string
					var createdAt time.Time
					require.NoError(t, tx.QueryRow(ctx, `SELECT data, previous_data,
						coalesce(user_id, ''), coalesce(api_key_id::text, ''),
						coalesce(org_id::text, ''), coalesce(tenant_id::text, ''), created_at
						FROM audit.audit_events WHERE "table" = $1 AND entity_id = $2 AND operation = $3`,
						table, id, operation).Scan(&data, &oldData, &user, &key, &org, &tenant, &createdAt))
					require.JSONEq(t, current, data, operation)
					if operation == "UPDATE" {
						require.NotNil(t, oldData)
						require.JSONEq(t, previous, *oldData)
					} else {
						require.Nil(t, oldData)
					}
					require.Equal(t, tc.user, user)
					require.Equal(t, tc.key, key)
					require.Equal(t, tc.org, org)
					require.Equal(t, tc.tenant, tenant)
					require.False(t, createdAt.IsZero())
					previous = current
				}
			})
		}
	}
}

func TestUserAndAPIKeyAuditMigrationRoundTrip(t *testing.T) {
	migration, err := os.ReadFile("../repositories/migrations/20261013100000_fix_user_api_key_audit_triggers.sql")
	require.NoError(t, err)
	up, down, ok := strings.Cut(string(migration), "-- +goose Down")
	require.True(t, ok)

	ctx := context.Background()
	tx, err := pgPool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `
INSERT INTO tenants (id, name) VALUES ('22222222-2222-2222-2222-222222222222', 'audit-test');
INSERT INTO organizations (id, name, tenant_id) VALUES
('11111111-1111-1111-1111-111111111111', 'audit-test', '22222222-2222-2222-2222-222222222222');`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT set_config('custom.current_user_id', 'audit-migration-actor', true)`)
	require.NoError(t, err)
	var auditFunction string
	require.NoError(t, tx.QueryRow(ctx, `SELECT pg_get_functiondef('global_audit()'::regprocedure)`).Scan(&auditFunction))

	// Down restores the prior behavior; Up must correct it again without changing
	// the audit function or rewriting previously recorded events.
	for _, step := range []struct {
		name, sql             string
		userEvents, keyEvents int
	}{
		{"Down", down, 2, 0},
		{"Up", up, 1, 1},
	} {
		var before, after int
		require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM audit.audit_events`).Scan(&before))
		_, err = tx.Exec(ctx, step.sql)
		require.NoError(t, err, step.name)
		require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM audit.audit_events`).Scan(&after))
		require.Equal(t, before, after)
		var currentFunction string
		require.NoError(t, tx.QueryRow(ctx, `SELECT pg_get_functiondef('global_audit()'::regprocedure)`).Scan(&currentFunction))
		require.Equal(t, auditFunction, currentFunction)

		for _, table := range []string{"users", "api_keys"} {
			id := uuid.New()
			query := `INSERT INTO users (id, email, role) VALUES ($1::uuid, $1::uuid::text || '@audit-round-trip.example.com', 1)`
			want := step.userEvents
			if table == "api_keys" {
				query = `INSERT INTO api_keys (id, org_id, prefix, role, key_hash)
					VALUES ($1, '11111111-1111-1111-1111-111111111111', 'round-trip', 1, $2)`
				want = step.keyEvents
				_, err = tx.Exec(ctx, query, id, []byte(id.String()))
			} else {
				_, err = tx.Exec(ctx, query, id)
			}
			require.NoError(t, err)
			var count int
			require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM audit.audit_events
				WHERE "table" = $1 AND entity_id = $2`, table, id).Scan(&count))
			require.Equal(t, want, count, step.name+"/"+table)
		}
	}
}
