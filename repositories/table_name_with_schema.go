package repositories

import (
	"github.com/checkmarble/marble-backend/models"

	"github.com/jackc/pgx/v5"
)

func tableNameWithSchema(transaction Transaction, tableName models.TableName) string {

	return pgx.Identifier.Sanitize([]string{
		transaction.DatabaseSchema().Schema,
		string(tableName),
	})
}
