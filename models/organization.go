package models

import (
	"net"

	"github.com/google/uuid"
)

type OrganizationEnvironment int

const (
	OrganizationEnvironmentUnknown OrganizationEnvironment = iota
	OrganizationEnvironmentProduction
	OrganizationEnvironmentDemo
)

func (e OrganizationEnvironment) String() string {
	switch e {
	case OrganizationEnvironmentProduction:
		return "production"
	case OrganizationEnvironmentDemo:
		return "demo"
	default:
		return "unknown"
	}
}

func ParseOrganizationEnvironment(s string) OrganizationEnvironment {
	switch s {
	case "production":
		return OrganizationEnvironmentProduction
	case "demo":
		return OrganizationEnvironmentDemo
	default:
		return OrganizationEnvironmentUnknown
	}
}

type Organization struct {
	Id uuid.UUID

	PublicId uuid.UUID

	// Name of the organization. Because this can be used to map to the organization's ingested data schema, it is unique and immutable.
	Name string

	WhitelistedSubnets []net.IPNet

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

	// Flag to enable Sentry session replay capture for this organization (used for test orgs).
	SentryReplayEnabled bool

	// Environment of the organization (production or demo). Used to skip Sentry cron monitoring for demo orgs.
	Environment OrganizationEnvironment
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
	Name        string
	Environment *OrganizationEnvironment
}

type UpdateOrganizationInput struct {
	DefaultScenarioTimezone *string
	ScreeningConfig         OrganizationOpenSanctionsConfigUpdateInput
	AutoAssignQueueLimit    *int
	SentryReplayEnabled     *bool
	Environment             *OrganizationEnvironment
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
