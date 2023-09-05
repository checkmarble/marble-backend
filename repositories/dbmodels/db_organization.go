package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBOrganizationResult struct {
	Id                         string `db:"id"`
	Name                       string `db:"name"`
	DatabaseName               string `db:"database_name"`
	DeletedAt                  *int   `db:"deleted_at"`
	ExportScheduledExecutionS3 string `db:"export_scheduled_execution_s3"`
}

const TABLE_ORGANIZATION = "organizations"

var ColumnsSelectOrganization = utils.ColumnList[DBOrganizationResult]()

func AdaptOrganization(db DBOrganizationResult) models.Organization {

	return models.Organization{
		Id:                         db.Id,
		Name:                       db.Name,
		DatabaseName:               db.DatabaseName,
		ExportScheduledExecutionS3: db.ExportScheduledExecutionS3,
	}
}

type DBUpdateOrganization struct {
	Name         *string `db:"name"`
	DatabaseName *string `db:"database_name"`
}
