package models

type Organization struct {
	Id                         string
	Name                       string
	DatabaseName               string
	ExportScheduledExecutionS3 string
}

type CreateOrganizationInput struct {
	Name         string
	DatabaseName string
}

type UpdateOrganizationInput struct {
	Id                         string
	Name                       *string
	DatabaseName               *string
	ExportScheduledExecutionS3 *string
}
