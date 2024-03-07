package dto

import "github.com/checkmarble/marble-backend/models"

type APIOrganization struct {
	Id                         string `json:"id"`
	Name                       string `json:"name"`
	ExportScheduledExecutionS3 string `json:"export_scheduled_execution_s3"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		Id:                         org.Id,
		Name:                       org.Name,
		ExportScheduledExecutionS3: org.ExportScheduledExecutionS3,
	}
}

type CreateOrganizationBodyDto struct {
	Name string `json:"name"`
}

type CreateOrganizationInputDto struct {
	Body *CreateOrganizationBodyDto `in:"body=json"`
}

type UpdateOrganizationBodyDto struct {
	Name                       *string `json:"name,omitempty"`
	ExportScheduledExecutionS3 *string `json:"export_scheduled_execution_s3,omitempty"`
}

type UpdateOrganizationInputDto struct {
	OrganizationId string                     `in:"path=organizationId"`
	Body           *UpdateOrganizationBodyDto `in:"body=json"`
}

type DeleteOrganizationInput struct {
	OrganizationId string `in:"path=organizationId"`
}
