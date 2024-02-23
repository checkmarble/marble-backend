package models

type Organization struct {
	Id                         string
	Name                       string
	ExportScheduledExecutionS3 string
}

type CreateOrganizationInput struct {
	Name string
}

type UpdateOrganizationInput struct {
	Id                         string
	ExportScheduledExecutionS3 *string
	Name                       *string
}
