package dbmodels

import (
	"marble/marble-backend/models"
	"marble/marble-backend/pg_repository"
)

type DBOrganizationResult struct {
	Id                         string `db:"id"`
	Name                       string `db:"name"`
	DatabaseName               string `db:"database_name"`
	DeletedAt                  *int   `db:"deleted_at"`
	ExportScheduledExecutionS3 string `db:"export_scheduled_execution_s3"`
}

const TABLE_ORGANIZATION = "organizations"

var ColumnsSelectOrganization = pg_repository.ColumnList[DBOrganizationResult]()

func AdaptOrganization(db DBOrganizationResult) models.Organization {

	return models.Organization{
		ID:                         db.Id,
		Name:                       db.Name,
		DatabaseName:               db.DatabaseName,
		ExportScheduledExecutionS3: db.ExportScheduledExecutionS3,
	}
}

type DBUpdateOrganization struct {
	Name         *string `db:"name"`
	DatabaseName *string `db:"database_name"`
}
