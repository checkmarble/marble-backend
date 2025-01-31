package models

import "time"

type OrganizationFeatureAccess struct {
	Id             string
	OrganizationId string
	TestRun        FeatureAccess
	Workflows      FeatureAccess
	Webhooks       FeatureAccess
	RuleSnoozes    FeatureAccess
	Roles          FeatureAccess
	Analytics      FeatureAccess
	Sanctions      FeatureAccess
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type DbStoredOrganizationFeatureAccess struct {
	Id             string
	OrganizationId string
	TestRun        FeatureAccess
	Sanctions      FeatureAccess
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UpdateOrganizationFeatureAccessInput struct {
	OrganizationId string
	TestRun        *FeatureAccess
	Sanctions      *FeatureAccess
}

func (f DbStoredOrganizationFeatureAccess) MergeWithLicenseEntitlement(l *LicenseEntitlements) OrganizationFeatureAccess {
	o := OrganizationFeatureAccess{
		Id:             f.Id,
		OrganizationId: f.OrganizationId,
		TestRun:        f.TestRun,
		Sanctions:      f.Sanctions,
		CreatedAt:      f.CreatedAt,
		UpdatedAt:      f.UpdatedAt,
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
	}

	return o
}
