package postgres

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type missingUserRow struct{}

func (missingUserRow) Scan(...any) error { return pgx.ErrNoRows }

type missingUserGetter struct{ t *testing.T }

func (db missingUserGetter) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	require.Contains(db.t, query, "u.tenant_id")
	require.NotContains(db.t, query, "organization_id")
	require.Contains(db.t, query, "u.deleted_at IS NULL")
	require.Equal(db.t, []any{"user"}, args)
	return missingUserRow{}
}

func TestUserByIDRejectsMissingOrDeletedIdentity(t *testing.T) {
	_, err := userByID(context.Background(), missingUserGetter{t}, "user")
	require.ErrorIs(t, err, models.NotFoundError)
}
