package repositories

import (
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// PostgreSQL's default NAMEDATALEN is 64 bytes, of which one byte is reserved for the trailing
// null. Identifiers longer than this are clipped to a valid multibyte boundary before lookup.
const postgresMaxIdentifierBytes = 63

func truncatePostgresIdentifier(value string) string {
	if len(value) <= postgresMaxIdentifierBytes {
		return value
	}

	end := postgresMaxIdentifierBytes
	for end > 0 && !utf8.ValidString(value[:end]) {
		end--
	}

	return value[:end]
}

func pgIdentifierWithSchema(exec Executor, tableName string, field ...string) string {
	input := []string{exec.DatabaseSchema().Schema, tableName}
	if len(field) > 0 {
		input = append(input, field[0])
	}
	return pgx.Identifier.Sanitize(input)
}

// pgClientDataIdentifierString quotes a value for inclusion in a statement that cannot take a
// parameter. Callers decide whether they need the logical name or its physical PostgreSQL form.
func pgClientDataIdentifierString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
