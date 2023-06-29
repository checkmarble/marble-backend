package dto

import "marble/marble-backend/models"

type APIOrganization struct {
	ID                         string `json:"id"`
	Name                       string `json:"name"`
	DatabaseName               string `json:"database_name"`
	ExportScheduledExecutionS3 string `json:"export_scheduled_execution_s3"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		ID:                         org.ID,
		Name:                       org.Name,
		DatabaseName:               org.DatabaseName,
		ExportScheduledExecutionS3: org.ExportScheduledExecutionS3,
	}
}

type CreateOrganizationBodyDto struct {
	Name         string `json:"name"`
	DatabaseName string `json:"databaseName"`
}

type CreateOrganizationInputDto struct {
	Body *CreateOrganizationBodyDto `in:"body=json"`
}

type UpdateOrganizationBodyDto struct {
	Name                       *string `json:"name,omitempty"`
	DatabaseName               *string `json:"databaseName,omitempty"`
	ExportScheduledExecutionS3 *string `json:"export_scheduled_execution_s3,omitempty"`
}

type UpdateOrganizationInputDto struct {
	OrgID string                     `in:"path=orgID"`
	Body  *UpdateOrganizationBodyDto `in:"body=json"`
}

type DeleteOrganizationInput struct {
	OrgID string `in:"path=orgID"`
}
