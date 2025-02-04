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

func (o OrganizationFeatureAccess) WithTestMode() OrganizationFeatureAccess {
	if o.TestRun == Restricted {
		o.TestRun = Test
	}
	if o.Workflows == Restricted {
		o.Workflows = Test
	}
	if o.Webhooks == Restricted {
		o.Webhooks = Test
	}
	if o.RuleSnoozes == Restricted {
		o.RuleSnoozes = Test
	}
	if o.Roles == Restricted {
		o.Roles = Test
	}
	if o.Analytics == Restricted {
		o.Analytics = Test
	}
	if o.Sanctions == Restricted {
		o.Sanctions = Test
	}
	return o
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

type FeaturesConfiguration struct {
	Webhooks  bool
	Sanctions bool
	Analytics bool
}

func (f DbStoredOrganizationFeatureAccess) MergeWithLicenseEntitlement(
	l LicenseEntitlements,
	c FeaturesConfiguration,
	hasTestMode bool,
) OrganizationFeatureAccess {
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

	// as an exception, if test mode is enambled (if the app is running with the firebase auth emulator), set all the features to "test"
	if hasTestMode {
		o = o.WithTestMode()
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

	return o
}
