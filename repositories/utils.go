package repositories

import (
	"context"
	"fmt"
	"strings"

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

type executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type auditCommands struct {
	query string
	args  []any
}

var sqlUpdateVerbs = []string{"insert", "update", "delete"}

func injectDbSessionConfig(ctx context.Context, exec executor, query string) (pgconn.CommandTag, error) {
	if query != "" {
		exitEarly := true
		for _, v := range sqlUpdateVerbs {
			if strings.Contains(strings.ToLower(query), v) {
				exitEarly = false
				break
			}
		}
		if exitEarly {
			return pgconn.NewCommandTag(""), nil
		}
	}

	cmds := []auditCommands{
		{"set_config($1, null, false)", []any{postgres_audit_user_id_parameter}},
		{"set_config($2, null, false)", []any{postgres_audit_api_key_id_parameter}},
		{"set_config($3, null, false)", []any{postgres_audit_org_id_parameter}},
	}

	if creds, ok := utils.CredentialsFromCtx(ctx); ok {
		switch {
		case creds.ActorIdentity.UserId != "":
			cmds = append(cmds, auditCommands{"SET_CONFIG($4, $5, false)", []any{
				postgres_audit_user_id_parameter, creds.ActorIdentity.UserId,
			}})
		case creds.ActorIdentity.ApiKeyId != "":
			cmds = append(cmds, auditCommands{"SET_CONFIG($4, $5, false)", []any{
				postgres_audit_api_key_id_parameter, creds.ActorIdentity.ApiKeyId,
			}})
		default:
			// We need to select dummy values so we simplify the arguments logic
			cmds = append(cmds, auditCommands{"$4, $5", []any{0, 0}})
		}

		cmds = append(cmds, auditCommands{"SET_CONFIG($6, $7, false)", []any{
			postgres_audit_org_id_parameter, creds.OrganizationId,
		}})
	}

	if len(cmds) > 0 {
		queries := make([]string, len(cmds))
		args := make([]any, 0)

		for idx, cmd := range cmds {
			queries[idx] = cmd.query
			args = append(args, cmd.args...)
		}

		if tag, err := exec.Exec(ctx, fmt.Sprintf("SELECT %s", strings.Join(queries, ",")), args...); err != nil {
			return tag, err
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
