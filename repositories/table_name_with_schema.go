package repositories

import (
	"github.com/checkmarble/marble-backend/models"

	"github.com/jackc/pgx/v5"
)

func tableNameWithSchema(exec Executor, tableName models.TableName) string {
	return pgx.Identifier.Sanitize([]string{
		exec.DatabaseSchema().Schema,
		string(tableName),
	})
}
