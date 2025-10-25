package models

import (
	"time"

	"github.com/google/uuid"
)

type OrganizationFeatureAccess struct {
	Id                  string
	OrganizationId      uuid.UUID
	TestRun             FeatureAccess `redis:"test_run"`
	Workflows           FeatureAccess `redis:"workflows"`
	Webhooks            FeatureAccess `redis:"webhooks"`
	RuleSnoozes         FeatureAccess `redis:"rule_snoozes"`
	Roles               FeatureAccess `redis:"roles"`
	Analytics           FeatureAccess `redis:"analytics"`
	Sanctions           FeatureAccess `redis:"sanctions"`
	NameRecognition     FeatureAccess `redis:"name_recognition"`
	CaseAutoAssign      FeatureAccess `redis:"case_auto_assign"`
	CaseAiAssist        FeatureAccess `redis:"case_ai_assis"`
	ContinuousScreening FeatureAccess `redis:"continuous_screening"`
	CreatedAt           time.Time
	UpdatedAt           time.Time

	// user-scoped, temporarily at least
	AiAssist FeatureAccess `redis:"ai_assist"`
}

type DbStoredOrganizationFeatureAccess struct {
	Id                  string
	OrganizationId      uuid.UUID
	TestRun             FeatureAccess
	Sanctions           FeatureAccess
	CaseAutoAssign      FeatureAccess
	CaseAiAssist        FeatureAccess
	ContinuousScreening FeatureAccess
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type UpdateOrganizationFeatureAccessInput struct {
	OrganizationId      uuid.UUID
	TestRun             *FeatureAccess
	Sanctions           *FeatureAccess
	CaseAiAssist        *FeatureAccess
	CaseAutoAssign      *FeatureAccess
	ContinuousScreening *FeatureAccess
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
		Id:                  f.Id,
		OrganizationId:      f.OrganizationId,
		TestRun:             f.TestRun,
		Sanctions:           f.Sanctions,
		NameRecognition:     f.Sanctions,
		CaseAutoAssign:      f.CaseAutoAssign,
		CaseAiAssist:        f.CaseAiAssist,
		ContinuousScreening: f.ContinuousScreening,
		CreatedAt:           f.CreatedAt,
		UpdatedAt:           f.UpdatedAt,
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
	if !l.ContinuousScreening {
		o.ContinuousScreening = Restricted
	}
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
	if !l.CaseAiAssist {
		o.CaseAiAssist = Restricted
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
	if o.ContinuousScreening.IsAllowed() && !c.Sanctions {
		o.ContinuousScreening = MissingConfiguration
	}

	if user != nil && user.AiAssistEnabled {
		o.AiAssist = Allowed
	}

	return o
}
