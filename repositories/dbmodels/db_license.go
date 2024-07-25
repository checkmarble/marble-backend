package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/guregu/null/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBLicense struct {
	Id                   string             `db:"id"`
	Key                  string             `db:"key"`
	CreatedAt            time.Time          `db:"created_at"`
	SuspendedAt          pgtype.Timestamptz `db:"suspended_at"`
	ExpirationDate       time.Time          `db:"expiration_date"`
	OrganizationName     string             `db:"name"`
	Description          string             `db:"description"`
	SsoEntitlement       bool               `db:"sso_entitlement"`
	WorkflowsEntitlement bool               `db:"workflows_entitlement"`
	AnalyticsEntitlement bool               `db:"analytics_entitlement"`
	DataEnrichment       bool               `db:"data_enrichment"`
	UserRoles            bool               `db:"user_roles"`
	Webhooks             bool               `db:"webhooks"`
}

const TABLE_LICENSES = "licenses"

var LicenseFields = utils.ColumnList[DBLicense]()

func AdaptLicense(db DBLicense) (models.License, error) {
	return models.License{
		Id:               db.Id,
		Key:              db.Key,
		CreatedAt:        db.CreatedAt,
		SuspendedAt:      null.NewTime(db.SuspendedAt.Time, db.SuspendedAt.Valid),
		ExpirationDate:   db.ExpirationDate,
		OrganizationName: db.OrganizationName,
		Description:      db.Description,
		LicenseEntitlements: models.LicenseEntitlements{
			Sso:            db.SsoEntitlement,
			Workflows:      db.WorkflowsEntitlement,
			Analytics:      db.AnalyticsEntitlement,
			DataEnrichment: db.DataEnrichment,
			UserRoles:      db.UserRoles,
			Webhooks:       db.Webhooks,
		},
	}, nil
}
