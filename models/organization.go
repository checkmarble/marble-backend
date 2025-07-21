package models

import "github.com/google/uuid"

type Organization struct {
	Id string

	PublicId uuid.UUID

	// Name of the organization. Because this can be used to map to the organization's ingested data schema, it is unique and immutable.
	Name string

	// Scenario id user for transfercheck. Internal marble use only. On a regular org, this should be null.
	TransferCheckScenarioId *string

	// Default timezone used during scenario execution to interpret timestamps, e.g. when extracting a date/time part from a timestamp.
	// Uses a IANA timezone validated with the go time std lib. "UTC" is used if not set.
	DefaultScenarioTimezone *string

	// Flag to enable AI case review.
	// Temporary simple flag before we activate more fine-grained workflows based on organization and inbox.
	AiCaseReviewEnabled bool

	OpenSanctionsConfig  OrganizationOpenSanctionsConfig
	AutoAssignQueueLimit int
}

// TODO: Add other organization-level configuration options
type OrganizationOpenSanctionsConfig struct {
	MatchThreshold int
	MatchLimit     int
}

type OrganizationOpenSanctionsConfigUpdateInput struct {
	MatchThreshold *int
	MatchLimit     *int
}

type CreateOrganizationInput struct {
	Name string
}

type UpdateOrganizationInput struct {
	Id                      string
	DefaultScenarioTimezone *string
	ScreeningConfig         OrganizationOpenSanctionsConfigUpdateInput
	AutoAssignQueueLimit    *int
}

type SeedOrgConfiguration struct {
	CreateGlobalAdminEmail string
	CreateOrgAdminEmail    string
	CreateOrgName          string
}

type InitOrgInput struct {
	OrgName    string
	AdminEmail string
}
