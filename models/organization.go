package models

type Organization struct {
	Id string

	// Name of the organization. Because this can be used to map to the organization's ingested data schema, it is unique and immutable.
	Name string

	// Scenario id user for transfercheck. Internal marble use only. On a regular org, this should be null.
	TransferCheckScenarioId *string

	// Default timezone used during scenario execution to interpret timestamps, e.g. when extracting a date/time part from a timestamp.
	// Uses a IANA timezone validated with the go time std lib. "UTC" is used if not set.
	DefaultScenarioTimezone *string

	// Flagged to use the main marble db (in a separate schema) for ingested data of the company,
	// even if a default external DB is configured in the CLIENT_DB_CONFIG_FILE.
	// Not relevant for on-premise users.
	// This can be deprecated later when all demo orgs created before this feature have been deleted or graduated
	// to a separate DB.
	// TODO: clean this up when it's no longuer used.
	UseMarbleDbSchemaAsDefault bool

	OpenSanctionsConfig OrganizationOpenSanctionsConfig
}

// TODO: Add other organization-level configuration options
type OrganizationOpenSanctionsConfig struct {
	Datasets       []string
	MatchThreshold int
	MatchLimit     int
}

func DefaultOrganizationOpenSanctionsConfig() OrganizationOpenSanctionsConfig {
	return OrganizationOpenSanctionsConfig{
		Datasets:       []string{},
		MatchThreshold: 70,
		MatchLimit:     20,
	}
}

type CreateOrganizationInput struct {
	Name string
}

type UpdateOrganizationInput struct {
	Id                      string
	DefaultScenarioTimezone *string
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
