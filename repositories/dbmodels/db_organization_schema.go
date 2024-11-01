package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DbOrganizationSchema struct {
	Id         string `db:"id"`
	OrgId      string `db:"org_id"`
	SchemaName string `db:"schema_name"`
}

const ORGANIZATION_SCHEMA_TABLE = "organizations_schema"

var OrganizationSchemaFields = utils.ColumnList[DbOrganizationSchema]()

func AdaptOrganizationSchema(db DbOrganizationSchema) (models.OrganizationSchema, error) {
	return models.OrganizationSchema{
		OrganizationId: db.OrgId,
		DatabaseSchema: models.DatabaseSchema{
			SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
			Schema:     db.SchemaName,
		},
	}, nil
}
