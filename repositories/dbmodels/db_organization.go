package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBOrganizationResult struct {
	Id                         string  `db:"id"`
	DeletedAt                  *int    `db:"deleted_at"`
	Name                       string  `db:"name"`
	TransferCheckScenarioId    *string `db:"transfer_check_scenario_id"`
	UseMarbleDbSchemaAsDefault bool    `db:"use_marble_db_schema_as_default"`
	DefaultScenarioTimezone    *string `db:"default_scenario_timezone"`
}

const TABLE_ORGANIZATION = "organizations"

var ColumnsSelectOrganization = utils.ColumnList[DBOrganizationResult]()

func AdaptOrganization(db DBOrganizationResult) (models.Organization, error) {
	return models.Organization{
		Id:                         db.Id,
		Name:                       db.Name,
		TransferCheckScenarioId:    db.TransferCheckScenarioId,
		UseMarbleDbSchemaAsDefault: db.UseMarbleDbSchemaAsDefault,
		DefaultScenarioTimezone:    db.DefaultScenarioTimezone,
		// TODO: Actually get it from the database
		OpenSanctionsConfig: models.DefaultOrganizationOpenSanctionsConfig(),
	}, nil
}
