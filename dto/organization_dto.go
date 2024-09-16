package dto

import "github.com/checkmarble/marble-backend/models"

type APIOrganization struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		Id:   org.Id,
		Name: org.Name,
	}
}

type CreateOrganizationBodyDto struct {
	Name string `json:"name"`
}

type UpdateOrganizationBodyDto struct {
	Name *string `json:"name,omitempty"`
}
