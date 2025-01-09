package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_ORGANIZATION_FEATURE_ACCESS = "organization_feature_access"

var SelectOrganizationFeatureAccessColumn = utils.ColumnList[DBOrganizationFeatureAccess]()

type DBOrganizationFeatureAccess struct {
	Id             string    `db:"id"`
	OrganizationId string    `db:"org_id"`
	TestRun        string    `db:"test_run"`
	Sanctions      string    `db:"sanctions"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

func AdaptOrganizationFeatureAccess(db DBOrganizationFeatureAccess) (models.DbStoredOrganizationFeatureAccess, error) {
	return models.DbStoredOrganizationFeatureAccess{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		TestRun:        models.FeatureAccessFrom(db.TestRun),
		Sanctions:      models.FeatureAccessFrom(db.Sanctions),
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}, nil
}
