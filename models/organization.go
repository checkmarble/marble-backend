package models

type Organization struct {
	Id                         string
	Name                       string
	ExportScheduledExecutionS3 string
	TransferCheckScenarioId    *string
}

type CreateOrganizationInput struct {
	Name string
}

type UpdateOrganizationInput struct {
	Id                         string
	ExportScheduledExecutionS3 *string
	Name                       *string
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
