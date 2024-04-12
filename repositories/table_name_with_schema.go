package repositories

import (
	"github.com/jackc/pgx/v5"
)

func tableNameWithSchema(exec Executor, tableName string) string {
	return pgx.Identifier.Sanitize([]string{exec.DatabaseSchema().Schema, tableName})
}
