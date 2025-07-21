package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5"
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
			if tag, err := exec.Exec(ctx, "SELECT SET_CONFIG($1, $2, false)",
				postgres_audit_user_id_parameter, creds.ActorIdentity.UserId); err != nil {
				return tag, err
			}
		} else if creds.ActorIdentity.ApiKeyId != "" {
			if tag, err := exec.Exec(ctx, "SELECT SET_CONFIG($1, $2, false)",
				postgres_audit_api_key_id_parameter, creds.ActorIdentity.ApiKeyId); err != nil {
				return tag, err
			}
		}

		if creds.OrganizationId != "" {
			if tag, err := exec.Exec(ctx, "SELECT SET_CONFIG($1, $2, false)",
				postgres_audit_org_id_parameter, creds.OrganizationId); err != nil {
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

// For countByHelper
type countByItem struct {
	Id    string
	Count int
}

// Helper function to count the number of items by org id, useful for metrics collector
// The function expects the query to return a list of ID and count, we return a map of ID to count and set 0 for IDs that don't have any items
// Example:
// SELECT org_id, count(*) as count FROM decisions WHERE org_id IN ($1) AND created_at >= $2 AND created_at < $3 GROUP BY org_id
func countByHelper(ctx context.Context, exec Executor, query squirrel.Sqlizer, byIds []string) (map[string]int, error) {
	counts, err := SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (countByItem, error) {
		var result countByItem
		err := row.Scan(&result.Id, &result.Count)
		if err != nil {
			return countByItem{}, err
		}
		return result, nil
	})
	if err != nil {
		return map[string]int{}, err
	}

	result := make(map[string]int, len(byIds))
	for _, count := range counts {
		result[count.Id] = count.Count
	}

	// Set 0 for IDs which don't have any items
	for _, byId := range byIds {
		if _, exists := result[byId]; !exists {
			result[byId] = 0
		}
	}

	return result, nil
}
