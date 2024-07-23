package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

// This DTO serves as both a DTO for serializing the models (in the API layer) and for deserializing
// the license validation response from the license server (at server startup outside the SaaS offering)
// This means we should be careful of not making breaking changes to it such as removing fields.
type LicenseEntitlements struct {
	Sso            bool `json:"sso"`
	Workflows      bool `json:"workflows"`
	Analytics      bool `json:"analytics"`
	DataEnrichment bool `json:"data_enrichment"`
	UserRoles      bool `json:"user_roles"`
	Webhooks       bool `json:"webhooks"`
}

func AdaptLicenseEntitlements(licenseEntitlements models.LicenseEntitlements) LicenseEntitlements {
	return LicenseEntitlements{
		Sso:            licenseEntitlements.Sso,
		Workflows:      licenseEntitlements.Workflows,
		Analytics:      licenseEntitlements.Analytics,
		DataEnrichment: licenseEntitlements.DataEnrichment,
		UserRoles:      licenseEntitlements.UserRoles,
		Webhooks:       licenseEntitlements.Webhooks,
	}
}

type License struct {
	Id                  string              `json:"id"`
	Key                 string              `json:"key"`
	CreatedAt           time.Time           `json:"created_at"`
	SuspendedAt         null.Time           `json:"suspended_at"`
	ExpirationDate      time.Time           `json:"expiration_date"`
	OrganizationName    string              `json:"organization_name"`
	Description         string              `json:"description"`
	LicenseEntitlements LicenseEntitlements `json:"license_entitlements"`
}

func AdaptLicenseDto(license models.License) License {
	return License{
		Id:                  license.Id,
		Key:                 license.Key,
		CreatedAt:           license.CreatedAt,
		SuspendedAt:         license.SuspendedAt,
		ExpirationDate:      license.ExpirationDate,
		OrganizationName:    license.OrganizationName,
		Description:         license.Description,
		LicenseEntitlements: AdaptLicenseEntitlements(license.LicenseEntitlements),
	}
}

// This DTO serves as both a DTO for serializing the models (in the API layer) and for deserializing
// the license validation response from the license server (at server startup outside the SaaS offering)
// This means we should be careful of not making breaking changes to it such as removing fields.
type LicenseValidation struct {
	LicenseValidationCode string `json:"license_validation_code"`
	LicenseEntitlements   `json:"license_entitlements"`
}

func AdaptLicenseValidationDto(licenseValidation models.LicenseValidation) LicenseValidation {
	return LicenseValidation{
		LicenseValidationCode: licenseValidation.LicenseValidationCode.String(),
		LicenseEntitlements:   AdaptLicenseEntitlements(licenseValidation.LicenseEntitlements),
	}
}

func AdaptLicenseValidation(licenseValidation LicenseValidation) models.LicenseValidation {
	return models.LicenseValidation{
		LicenseValidationCode: models.LicenseValidationCodeFromString(
			licenseValidation.LicenseValidationCode),
		LicenseEntitlements: models.LicenseEntitlements(licenseValidation.LicenseEntitlements),
	}
}

type CreateLicenseBody struct {
	ExpirationDate      time.Time `json:"expiration_date"`
	OrganizationName    string    `json:"organization_name"`
	Description         string    `json:"description"`
	LicenseEntitlements `json:"license_entitlements"`
}

func AdaptCreateLicenseInput(body CreateLicenseBody) models.CreateLicenseInput {
	return models.CreateLicenseInput{
		ExpirationDate:      body.ExpirationDate,
		OrganizationName:    body.OrganizationName,
		Description:         body.Description,
		LicenseEntitlements: models.LicenseEntitlements(body.LicenseEntitlements),
	}
}

type UpdateLicenseBody struct {
	Suspend             null.Bool                       `json:"suspend"`
	ExpirationDate      null.Time                       `json:"expiration_date"`
	OrganizationName    null.String                     `json:"organization_name"`
	Description         null.String                     `json:"description"`
	LicenseEntitlements null.Value[LicenseEntitlements] `json:"license_entitlements"`
}

func AdaptUpdateLicenseInput(licenseId string, body UpdateLicenseBody) models.UpdateLicenseInput {
	updateLicenseInput := models.UpdateLicenseInput{
		Id:               licenseId,
		Suspend:          body.Suspend,
		ExpirationDate:   body.ExpirationDate,
		OrganizationName: body.OrganizationName,
		Description:      body.Description,
	}
	if body.LicenseEntitlements.Valid {
		updateLicenseInput.LicenseEntitlements =
			null.ValueFrom(models.LicenseEntitlements(body.LicenseEntitlements.V))
	}

	return updateLicenseInput
}
