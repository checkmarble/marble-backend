package repositories

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
