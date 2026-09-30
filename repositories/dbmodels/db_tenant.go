package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

const TABLE_TENANTS = "tenants"

type DBTenant struct {
	Id   uuid.UUID `db:"id"`
	Name string    `db:"name"`
}

var TenantColumns = utils.ColumnList[DBTenant]()

func AdaptTenant(db DBTenant) (models.Tenant, error) {
	return models.Tenant{Id: db.Id, Name: db.Name}, nil
}
