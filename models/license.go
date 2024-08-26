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
		},
	}
}

func NewDevLicense() LicenseValidation {
	return LicenseValidation{
		LicenseValidationCode: VALID,
		LicenseEntitlements: LicenseEntitlements{
			Sso:            true,
			Workflows:      true,
			Analytics:      true,
			DataEnrichment: true,
			UserRoles:      true,
			Webhooks:       false,
			RuleSnoozes:    true,
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
	IsDevEnvironment       bool
}
