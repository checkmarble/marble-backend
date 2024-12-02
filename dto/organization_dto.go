package dto

import "github.com/checkmarble/marble-backend/models"

type APIOrganization struct {
	Id                      string  `json:"id"`
	Name                    string  `json:"name"`
	DefaultScenarioTimezone *string `json:"default_scenario_timezone"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		Id:                      org.Id,
		Name:                    org.Name,
		DefaultScenarioTimezone: org.DefaultScenarioTimezone,
	}
}

type CreateOrganizationBodyDto struct {
	Name                    string  `json:"name"`
	DefaultScenarioTimezone *string `json:"default_scenario_timezone"`
}

type UpdateOrganizationBodyDto struct {
	DefaultScenarioTimezone *string `json:"default_scenario_timezone,omitempty"`
}
