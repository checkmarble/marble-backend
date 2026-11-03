package repositories

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type autoAssignmentQueryRecorder struct {
	Executor
	sql  string
	args []any
	err  error
}

func (e *autoAssignmentQueryRecorder) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	e.sql, e.args = sql, args
	return nil, e.err
}

func TestAutoAssignmentOrganizationAccess(t *testing.T) {
	t.Parallel()
	orgID, inboxID := uuid.New(), uuid.New()
	tests := []struct {
		name string
		run  func(*MarbleDbRepository, Executor) error
		args []any
	}{
		{
			name: "users",
			run: func(repo *MarbleDbRepository, exec Executor) error {
				_, err := repo.FindAutoAssignableUsers(t.Context(), exec, orgID, 10)
				return err
			},
			args: []any{orgID, 10},
		},
		{
			name: "next user for inbox",
			run: func(repo *MarbleDbRepository, exec Executor) error {
				_, err := repo.FindNextAutoAssignableUserForInbox(t.Context(), exec, orgID, inboxID, 10)
				return err
			},
			args: []any{orgID, inboxID, 10},
		},
		{
			name: "cases",
			run: func(repo *MarbleDbRepository, exec Executor) error {
				_, err := repo.FindAutoAssignableCases(t.Context(), exec, orgID, 10)
				return err
			},
			args: []any{orgID, 10},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			queryErr := errors.New("query stopped before execution")
			exec := &autoAssignmentQueryRecorder{err: queryErr}
			require.ErrorIs(t, tt.run(&MarbleDbRepository{}, exec), queryErr)
			sql := strings.Join(strings.Fields(exec.sql), " ")
			require.Equal(t, tt.args, exec.args)
			require.NotContains(t, sql, "u.organization_id =")
			require.NotContains(t, sql, "= u.organization_id")
			require.Contains(t, sql, "i.id = iu.inbox_id and i.organization_id = $1")
			require.Contains(t, sql, "c.org_id = $1 and c.assigned_to = u.id")
			require.Contains(t, sql, "exists ( select 1 from active_grants g")
			require.Contains(t, sql, "g.principal_type = 'user' and g.principal_authority = 'marble'")
			require.Contains(t, sql, "g.principal_id = u.id::text and g.organization_id = $1")
			require.Contains(t, sql, "g.role in ('VIEWER', 'ANALYST', 'BUILDER', 'PUBLISHER', 'ADMIN')")
			require.Contains(t, sql, "u.deleted_at is null")
			require.Contains(t, sql, "iu.auto_assignable")
			require.Contains(t, sql, "uu.org_id = $1")
		})
	}
}
