package models

import "time"

type OrganizationFeatureAccess struct {
	Id             string
	OrganizationId string
	TestRun        FeatureAccess
	Workflows      FeatureAccess
	Webhooks       FeatureAccess
	RuleSnoozed    FeatureAccess
	Roles          FeatureAccess
	Analytics      FeatureAccess
	Sanctions      FeatureAccess
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UpdateOrganizationFeatureAccessInput struct {
	TestRun     FeatureAccess
	Workflows   FeatureAccess
	Webhooks    FeatureAccess
	RuleSnoozed FeatureAccess
	Roles       FeatureAccess
	Analytics   FeatureAccess
	Sanctions   FeatureAccess
}
