package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_ORGANIZATION_ENTITLEMENTS = "organization_entitlements"

var SelectOrganizationEntitlementColumn = utils.ColumnList[models.OrganizationEntitlement]()

type DBOrganizationEntitlement struct {
	Id             string    `db:"id"`
	OrganizationId string    `db:"organization_id"`
	FeatureId      string    `db:"feature_id"`
	Availability   string    `db:"availability"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

func AdaptOrganizationEntitlement(db DBOrganizationEntitlement) (models.OrganizationEntitlement, error) {
	return models.OrganizationEntitlement{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		FeatureId:      db.FeatureId,
		Availability:   models.FeatureAvailabilityFrom(db.Availability),
	}, nil
}

type DBOrganizationEntitlementCreateInput struct {
	Id             string `db:"id"`
	OrganizationId string `db:"organization_id"`
	FeatureId      string `db:"feature_id"`
	Availability   string `db:"availability"`
}

func AdaptOrganizationEntitlementCreateInput(db DBOrganizationEntitlementCreateInput) models.CreateOrganizationEntitlementInput {
	return models.CreateOrganizationEntitlementInput{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		FeatureId:      db.FeatureId,
		Availability:   models.FeatureAvailabilityFrom(db.Availability),
	}
}

type DBOrganizationEntitlementUpdateInput struct {
	Id             string `db:"id"`
	OrganizationId string `db:"organization_id"`
	FeatureId      string `db:"feature_id"`
	Availability   string `db:"availability"`
}

func AdaptOrganizationEntitlementUpdateInput(db DBOrganizationEntitlementUpdateInput) models.UpdateOrganizationEntitlementInput {
	return models.UpdateOrganizationEntitlementInput{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		FeatureId:      db.FeatureId,
		Availability:   models.FeatureAvailabilityFrom(db.Availability),
	}
}
