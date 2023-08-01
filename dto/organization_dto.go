package dto

import "marble/marble-backend/models"

type APIOrganization struct {
	Id                         string `json:"id"`
	Name                       string `json:"name"`
	DatabaseName               string `json:"database_name"`
	ExportScheduledExecutionS3 string `json:"export_scheduled_execution_s3"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		Id:                         org.Id,
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
	OrganizationId string                     `in:"path=organizationId"`
	Body           *UpdateOrganizationBodyDto `in:"body=json"`
}

type DeleteOrganizationInput struct {
	OrganizationId string `in:"path=organizationId"`
}
