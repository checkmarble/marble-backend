package repositories

import (
	"strings"

	"github.com/jackc/pgx/v5"
)

func pgIdentifierWithSchema(exec Executor, tableName string, field ...string) string {
	input := []string{exec.DatabaseSchema().Schema, tableName}
	if len(field) > 0 {
		input = append(input, field[0])
	}
	return pgx.Identifier.Sanitize(input)
}

// pgClientDataIdentifierString quotes a value for inclusion in a statement that cannot take a parameter. Only
// the record type and field names go through it, both of which the data model has already
// constrained to `^[a-z][a-z0-9_]{0,62}$`; the quoting is what keeps that from being the only
// thing standing between the data model and injected SQL.
func pgClientDataIdentifierString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
