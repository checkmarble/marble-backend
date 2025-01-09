package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type APIOrganizationFeatureAccess struct {
	TestRun     string `json:"test_run"`
	Workflows   string `json:"workflows"`
	Webhooks    string `json:"webhooks"`
	RuleSnoozes string `json:"rule_snoozes"`
	Roles       string `json:"roles"`
	Analytics   string `json:"analytics"`
	Sanctions   string `json:"sanctions"`
}

func AdaptOrganizationFeatureAccessDto(f models.OrganizationFeatureAccess) APIOrganizationFeatureAccess {
	return APIOrganizationFeatureAccess{
		TestRun:     f.TestRun.String(),
		Workflows:   f.Workflows.String(),
		Webhooks:    f.Webhooks.String(),
		RuleSnoozes: f.RuleSnoozes.String(),
		Roles:       f.Roles.String(),
		Analytics:   f.Analytics.String(),
		Sanctions:   f.Sanctions.String(),
	}
}

type UpdateOrganizationFeatureAccessBodyDto struct {
	TestRun   string `json:"test_run"`
	Sanctions string `json:"sanctions"`
}
