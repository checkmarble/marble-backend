package models

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"
)

type LicenseValidationCode int

const (
	VALID LicenseValidationCode = iota
	EXPIRED
	NOT_FOUND
	OVERDUE
	SUSPENDED
)

// Provide a string value for each outcome
func (o LicenseValidationCode) String() string {
	switch o {
	case VALID:
		return "VALID"
	case EXPIRED:
		return "EXPIRED"
	case NOT_FOUND:
		return "NOT_FOUND"
	case OVERDUE:
		return "OVERDUE"
	case SUSPENDED:
		return "SUSPENDED"
	}
	return "NOT_FOUND"
}

func LicenseValidationCodeFromString(s string) LicenseValidationCode {
	switch s {
	case "VALID":
		return VALID
	case "EXPIRED":
		return EXPIRED
	case "NOT_FOUND":
		return NOT_FOUND
	case "OVERDUE":
		return OVERDUE
	case "SUSPENDED":
		return SUSPENDED
	}
	return NOT_FOUND
}

type LicenseEntitlements struct {
	Sso            bool
	Workflows      bool
	Analytics      bool
	DataEnrichment bool
	UserRoles      bool
	Webhooks       bool
	RuleSnoozes    bool
	TestRun        bool
	Sanctions      bool
}

func (l *LicenseEntitlements) MergeWithFeatureAccess(f DbStoredOrganizationFeatureAccess) OrganizationFeatureAccess {
	o := OrganizationFeatureAccess{
		Id:             f.Id,
		OrganizationId: f.OrganizationId,
		TestRun:        f.TestRun,
		Sanctions:      f.Sanctions,
		CreatedAt:      f.CreatedAt,
		UpdatedAt:      f.UpdatedAt,
	}
	// Add the feature accesses computed directly from the license entitlements
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

	// set to "test" if the feature access overrides the license
	if f.TestRun == Test {
		o.TestRun = Test
	}
	if f.Sanctions == Test {
		o.Sanctions = Test
	}

	return o
}

type LicenseValidation struct {
	LicenseValidationCode
	LicenseEntitlements
}

func NewFullLicense() LicenseValidation {
	return LicenseValidation{
		LicenseValidationCode: VALID,
		LicenseEntitlements: LicenseEntitlements{
			Sso:            true,
			Workflows:      true,
			Analytics:      true,
			DataEnrichment: true,
			UserRoles:      true,
			Webhooks:       true,
			RuleSnoozes:    true,
			TestRun:        true,
			Sanctions:      true,
		},
	}
}

func NewNotFoundLicense() LicenseValidation {
	return LicenseValidation{
		LicenseValidationCode: NOT_FOUND,
	}
}

type License struct {
	Id               string
	Key              string
	CreatedAt        time.Time
	SuspendedAt      null.Time
	ExpirationDate   time.Time
	OrganizationName string
	Description      string
	LicenseEntitlements
}

type CreateLicenseInput struct {
	ExpirationDate   time.Time
	OrganizationName string
	Description      string
	LicenseEntitlements
}

type UpdateLicenseInput struct {
	Id                  string
	Suspend             null.Bool
	ExpirationDate      null.Time
	OrganizationName    null.String
	Description         null.String
	LicenseEntitlements null.Value[LicenseEntitlements]
}

func (l *UpdateLicenseInput) Validate() error {
	if !l.Suspend.Valid && !l.ExpirationDate.Valid && !l.OrganizationName.Valid &&
		!l.Description.Valid && !l.LicenseEntitlements.Valid {
		return errors.Wrap(BadParameterError, "at least one field must be set")
	}

	return nil
}

type LicenseConfiguration struct {
	LicenseKey             string
	KillIfReadLicenseError bool
}
