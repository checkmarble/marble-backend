package models

const BLANK_ORGANIZATION_ID = "c5a35fbd-6266-46ef-8d44-9310c14bccd6"

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
