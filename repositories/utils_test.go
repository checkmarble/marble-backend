package repositories

import (
	"context"
	"strings"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type textEncodingExecutor struct{}

func (textEncodingExecutor) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	// set_config takes text parameters; PostgreSQL also infers text for the
	// standalone placeholders used when there is no actor identity.
	types := pgtype.NewMap()
	for i, arg := range args {
		if _, err := types.Encode(pgtype.TextOID, pgtype.TextFormatCode, arg, nil); err != nil {
			return pgconn.CommandTag{}, errors.Errorf("failed to encode args[%d]: %w", i, err)
		}
	}
	return pgconn.NewCommandTag("SELECT 1"), nil
}

func TestInjectDbSessionConfig_TextEncoding(t *testing.T) {
	for _, tc := range []struct {
		name  string
		creds models.Credentials
	}{
		{name: "global system job", creds: models.Credentials{}},
		{name: "organization system job", creds: models.Credentials{OrganizationId: uuid.New()}},
		{name: "user", creds: models.Credentials{ActorIdentity: models.Identity{UserId: "user-id"}}},
		{name: "api key", creds: models.Credentials{ActorIdentity: models.Identity{ApiKeyId: "api-key-id"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), utils.ContextKeyCredentials, tc.creds)
			_, err := injectDbSessionConfig(ctx, textEncodingExecutor{}, "SELECT updated_at FROM continuous_screening_configs")
			require.NoError(t, err)
		})
	}
}

type recordingExecutor struct {
	queries []string
}

func (e *recordingExecutor) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	e.queries = append(e.queries, sql)
	return pgconn.NewCommandTag(""), nil
}

func TestInjectDbSessionConfig(t *testing.T) {
	t.Run("sets audit configuration on writes and transactions", func(t *testing.T) {
		for _, query := range []string{"UPDATE decisions SET review_status = 'ok'", ""} {
			exec := &recordingExecutor{}

			_, err := injectDbSessionConfig(context.Background(), exec, query)

			assert.NoError(t, err)
			assert.Len(t, exec.queries, 1)
			assert.True(t, strings.HasPrefix(exec.queries[0], "SELECT set_config("))
		}
	})

	t.Run("skips audit configuration when disabled in context", func(t *testing.T) {
		ctx := WithoutAudit(context.Background())

		for _, query := range []string{"UPDATE decisions SET review_status = 'ok'", ""} {
			exec := &recordingExecutor{}

			_, err := injectDbSessionConfig(ctx, exec, query)

			assert.NoError(t, err)
			assert.Empty(t, exec.queries)
		}
	})
}
