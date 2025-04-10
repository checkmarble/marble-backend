package repositories

import (
	"github.com/jackc/pgx/v5"
)

func pgIdentifierWithSchema(exec Executor, tableName string, field ...string) string {
	input := []string{exec.DatabaseSchema().Schema, tableName}
	if len(field) > 0 {
		input = append(input, field[0])
	}
	return pgx.Identifier.Sanitize(input)
}
