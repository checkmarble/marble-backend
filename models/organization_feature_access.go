package models

import (
	"time"
)

type OrganizationFeatureAccess struct {
	Id              string
	OrganizationId  string
	TestRun         FeatureAccess
	Workflows       FeatureAccess
	Webhooks        FeatureAccess
	RuleSnoozes     FeatureAccess
	Roles           FeatureAccess
	Analytics       FeatureAccess
	Sanctions       FeatureAccess
	NameRecognition FeatureAccess
	CaseAutoAssign  FeatureAccess
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// user-scoped, temporarily at least
	AiAssist FeatureAccess
}

type DbStoredOrganizationFeatureAccess struct {
	Id             string
	OrganizationId string
	TestRun        FeatureAccess
	Sanctions      FeatureAccess
	CaseAutoAssign FeatureAccess
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UpdateOrganizationFeatureAccessInput struct {
	OrganizationId string
	TestRun        *FeatureAccess
	Sanctions      *FeatureAccess
}

type FeaturesConfiguration struct {
	Webhooks        bool
	Sanctions       bool
	NameRecognition bool
	Analytics       bool
}

func (f DbStoredOrganizationFeatureAccess) MergeWithLicenseEntitlement(
	l LicenseEntitlements,
	c FeaturesConfiguration,
	user *User,
) OrganizationFeatureAccess {
	o := OrganizationFeatureAccess{
		Id:              f.Id,
		OrganizationId:  f.OrganizationId,
		TestRun:         f.TestRun,
		Sanctions:       f.Sanctions,
		NameRecognition: f.Sanctions,
		CaseAutoAssign:  f.CaseAutoAssign,
		CreatedAt:       f.CreatedAt,
		UpdatedAt:       f.UpdatedAt,
	}

	// First, set the feature accesses to "allowed" if the license allows it
	if l.Analytics {
		o.Analytics = Allowed
	}
	if l.Webhooks {
		o.Webhooks = Allowed
	}
	if l.Workflows {
		o.Workflows = Allowed
	}
	if l.RuleSnoozes {
		o.RuleSnoozes = Allowed
	}
	if l.UserRoles {
		o.Roles = Allowed
	}

	// remove the feature accesses that are not allowed by the license
	if !l.TestRun {
		o.TestRun = Restricted
	}
	if !l.Sanctions {
		o.Sanctions = Restricted
		o.NameRecognition = Restricted
	}
	if !l.CaseAutoAssign {
		o.CaseAutoAssign = Restricted
	}

	// remove the feature accesses that are not allowed by the configuration
	if o.Analytics.IsAllowed() && !c.Analytics {
		o.Analytics = MissingConfiguration
	}
	if o.Webhooks.IsAllowed() && !c.Webhooks {
		o.Webhooks = MissingConfiguration
	}
	if o.Sanctions.IsAllowed() && !c.Sanctions {
		o.Sanctions = MissingConfiguration
	}
	if o.NameRecognition.IsAllowed() && !c.NameRecognition {
		o.NameRecognition = MissingConfiguration
	}

	if user != nil && user.AiAssistEnabled {
		o.AiAssist = Allowed
	}

	return o
}
