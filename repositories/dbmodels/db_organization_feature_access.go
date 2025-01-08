package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

const TABLE_ORGANIZATION_FEATURE_ACCESS = "organization_feature_access"

var SelectOrganizationFeatureAccessColumn = utils.ColumnList[models.OrganizationFeatureAccess]()

type DBOrganizationFeatureAccess struct {
	Id             string    `db:"id"`
	OrganizationId string    `db:"org_id"`
	TestRun        string    `db:"test_run"`
	Workflows      string    `db:"workflows"`
	Webhooks       string    `db:"webhooks"`
	RuleSnoozed    string    `db:"rule_snoozed"`
	Roles          string    `db:"roles"`
	Analytics      string    `db:"analytics"`
	Sanctions      string    `db:"sanctions"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

func AdaptOrganizationFeatureAccess(db DBOrganizationFeatureAccess) (models.OrganizationFeatureAccess, error) {
	return models.OrganizationFeatureAccess{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		TestRun:        models.FeatureAccessFrom(db.TestRun),
		Workflows:      models.FeatureAccessFrom(db.Workflows),
		Webhooks:       models.FeatureAccessFrom(db.Webhooks),
		RuleSnoozed:    models.FeatureAccessFrom(db.RuleSnoozed),
		Roles:          models.FeatureAccessFrom(db.Roles),
		Analytics:      models.FeatureAccessFrom(db.Analytics),
		Sanctions:      models.FeatureAccessFrom(db.Sanctions),
	}, nil
}

type DBOrganizationFeatureAccessUpdateInput struct {
	Id             string `db:"id"`
	OrganizationId string `db:"organization_id"`
	TestRun        string `db:"test_run"`
	Workflows      string `db:"workflows"`
	Webhooks       string `db:"webhooks"`
	RuleSnoozed    string `db:"rule_snoozed"`
	Roles          string `db:"roles"`
	Analytics      string `db:"analytics"`
	Sanctions      string `db:"sanctions"`
}

func AdaptOrganizationFeatureAccessUpdateInput(db DBOrganizationFeatureAccessUpdateInput) models.UpdateOrganizationFeatureAccessInput {
	return models.UpdateOrganizationFeatureAccessInput{
		OrganizationId: db.OrganizationId,
		TestRun:        models.FeatureAccessFrom(db.TestRun),
		Workflows:      models.FeatureAccessFrom(db.Workflows),
		Webhooks:       models.FeatureAccessFrom(db.Webhooks),
		RuleSnoozed:    models.FeatureAccessFrom(db.RuleSnoozed),
		Roles:          models.FeatureAccessFrom(db.Roles),
		Analytics:      models.FeatureAccessFrom(db.Analytics),
		Sanctions:      models.FeatureAccessFrom(db.Sanctions),
	}
}
