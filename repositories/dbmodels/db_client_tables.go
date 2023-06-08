package dbmodels

import (
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
)

type DBClientTables struct {
	Id         string `db:"id"`
	OrgId      string `db:"org_id"`
	SchemaName string `db:"schema_name"`
}

const TABLE_CLIENT_TABLES = "client_tables"

var ClientTablesFields = pg_repository.ColumnList[DBClientTables]()

func AdaptClientTable(db DBClientTables) models.ClientTables {
	return models.ClientTables{
		OrganizationId: db.OrgId,
		DatabaseSchema: models.DatabaseSchema{
			SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
			Database:   models.DATABASE_MARBLE,
			Schema:     db.SchemaName,
		},
	}
}
