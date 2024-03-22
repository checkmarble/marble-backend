package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBOrganizationResult struct {
	Id                         string  `db:"id"`
	DeletedAt                  *int    `db:"deleted_at"`
	ExportScheduledExecutionS3 string  `db:"export_scheduled_execution_s3"`
	Name                       string  `db:"name"`
	TransferCheckScenarioId    *string `db:"transfer_check_scenario_id"`
}

const TABLE_ORGANIZATION = "organizations"

var ColumnsSelectOrganization = utils.ColumnList[DBOrganizationResult]()

func AdaptOrganization(db DBOrganizationResult) (models.Organization, error) {
	return models.Organization{
		Id:                         db.Id,
		ExportScheduledExecutionS3: db.ExportScheduledExecutionS3,
		Name:                       db.Name,
		TransferCheckScenarioId:    db.TransferCheckScenarioId,
	}, nil
}
