package models

type Organization struct {
	Id                      string
	Name                    string
	TransferCheckScenarioId *string
	DefaultScenarioTimezone *string

	// Flagged to use the main marble db (in a separate schema) for ingested data of the company,
	// even if a default external DB is configured in the CLIENT_DB_CONFIG_FILE.
	// Not relevant for on-premise users.
	// This can be deprecated later when all demo orgs created before this feature have been deleted or graduated
	// to a separate DB.
	// TODO: clean this up when it's no longuer used.
	UseMarbleDbSchemaAsDefault bool
}

type CreateOrganizationInput struct {
	Name string
}

type UpdateOrganizationInput struct {
	Id   string
	Name *string
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
