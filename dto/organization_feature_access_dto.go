package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APIOrganizationFeatureAccess struct {
	Id             string    `json:"id"`
	OrganizationId string    `json:"organization_id"`
	TestRun        string    `json:"test_run"`
	Workflows      string    `json:"workflows"`
	Webhooks       string    `json:"webhooks"`
	RuleSnoozed    string    `json:"rule_snoozed"`
	Roles          string    `json:"roles"`
	Analytics      string    `json:"analytics"`
	Sanctions      string    `json:"sanctions"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func AdaptOrganizationFeatureAccessDto(f models.OrganizationFeatureAccess) APIOrganizationFeatureAccess {
	return APIOrganizationFeatureAccess{
		Id:             f.Id,
		OrganizationId: f.OrganizationId,
		TestRun:        f.TestRun.String(),
		Workflows:      f.Workflows.String(),
		Webhooks:       f.Webhooks.String(),
		RuleSnoozed:    f.RuleSnoozed.String(),
		Roles:          f.Roles.String(),
		Analytics:      f.Analytics.String(),
		Sanctions:      f.Sanctions.String(),
		CreatedAt:      f.CreatedAt,
		UpdatedAt:      f.UpdatedAt,
	}
}

type UpdateOrganizationFeatureAccessBodyDto struct {
	OrganizationId string `json:"organization_id" binding:"required"`
	TestRun        string `json:"test_run" binding:"required"`
	Workflows      string `json:"workflows" binding:"required"`
	Webhooks       string `json:"webhooks" binding:"required"`
	RuleSnoozed    string `json:"rule_snoozed" binding:"required"`
	Roles          string `json:"roles" binding:"required"`
	Analytics      string `json:"analytics" binding:"required"`
	Sanctions      string `json:"sanctions" binding:"required"`
}
