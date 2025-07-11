package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	postgres_audit_org_id_parameter     = "custom.current_org_id"
	postgres_audit_user_id_parameter    = "custom.current_user_id"
	postgres_audit_api_key_id_parameter = "custom.current_api_key_id"
)

type errorRow struct {
	err error
}

func (e errorRow) Scan(args ...any) error {
	return e.err
}

func injectDbSessionConfig(ctx context.Context, exec TransactionOrPool) (pgconn.CommandTag, error) {
	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		if creds.ActorIdentity.UserId != "" {
			if tag, err := exec.Exec(ctx, "SELECT SET_CONFIG($1, $2, false)", postgres_audit_user_id_parameter, creds.ActorIdentity.UserId); err != nil {
				return tag, err
			}
		} else if creds.ActorIdentity.ApiKeyId != "" {
			if tag, err := exec.Exec(ctx, "SELECT SET_CONFIG($1, $2, false)", postgres_audit_api_key_id_parameter, creds.ActorIdentity.ApiKeyId); err != nil {
				return tag, err
			}
		}

		if creds.OrganizationId != "" {
			if tag, err := exec.Exec(ctx, "SELECT SET_CONFIG($1, $2, false)", postgres_audit_org_id_parameter, creds.OrganizationId); err != nil {
				return tag, err
			}
		}
	}

	return pgconn.NewCommandTag(""), nil
}

func columnsNames(tablename string, fields []string) []string {
	return pure_utils.Map(fields, func(f string) string {
		return fmt.Sprintf("%s.%s", tablename, f)
	})
}
