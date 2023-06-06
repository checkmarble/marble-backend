package dbmodels

import (
	"marble/marble-backend/models"
)

type DBOrganizationResult struct {
	Id           string `db:"id"`
	Name         string `db:"name"`
	DatabaseName string `db:"database_name"`
	DeletedAt    *int   `db:"deleted_at"`
}

const TABLE_ORGANIZATION = "organizations"

var OrganizationFields = []string{"id", "name", "database_name", "deleted_at"}

func AdaptOrganization(db DBOrganizationResult) models.Organization {

	return models.Organization{
		ID:           db.Id,
		Name:         db.Name,
		DatabaseName: db.DatabaseName,
	}
}

type DBUpdateOrganization struct {
	Name         *string `db:"name"`
	DatabaseName *string `db:"database_name"`
}
