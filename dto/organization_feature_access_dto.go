package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type APIOrganizationFeatureAccess struct {
	TestRun     string `json:"test_run"`
	Workflows   string `json:"workflows"`
	Webhooks    string `json:"webhooks"`
	RuleSnoozed string `json:"rule_snoozed"`
	Roles       string `json:"roles"`
	Analytics   string `json:"analytics"`
	Sanctions   string `json:"sanctions"`
}

func AdaptOrganizationFeatureAccessDto(f models.OrganizationFeatureAccess) APIOrganizationFeatureAccess {
	return APIOrganizationFeatureAccess{
		TestRun:     f.TestRun.String(),
		Workflows:   f.Workflows.String(),
		Webhooks:    f.Webhooks.String(),
		RuleSnoozed: f.RuleSnoozed.String(),
		Roles:       f.Roles.String(),
		Analytics:   f.Analytics.String(),
		Sanctions:   f.Sanctions.String(),
	}
}

type UpdateOrganizationFeatureAccessBodyDto struct {
	TestRun     string `json:"test_run" binding:"required"`
	Workflows   string `json:"workflows" binding:"required"`
	Webhooks    string `json:"webhooks" binding:"required"`
	RuleSnoozed string `json:"rule_snoozed" binding:"required"`
	Roles       string `json:"roles" binding:"required"`
	Analytics   string `json:"analytics" binding:"required"`
	Sanctions   string `json:"sanctions" binding:"required"`
}
