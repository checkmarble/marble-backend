package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type APIOrganization struct {
	Id                      string  `json:"id"`
	Name                    string  `json:"name"`
	DefaultScenarioTimezone *string `json:"default_scenario_timezone"`
	SanctionsThreshold      int     `json:"sanctions_threshold"`
	SanctionsLimit          int     `json:"sanctions_limit"`
	AutoAssignQueueLimit    int     `json:"auto_assign_queue_limit"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		Id:                      org.Id,
		Name:                    org.Name,
		DefaultScenarioTimezone: org.DefaultScenarioTimezone,
		SanctionsThreshold:      org.OpenSanctionsConfig.MatchThreshold,
		SanctionsLimit:          org.OpenSanctionsConfig.MatchLimit,
		AutoAssignQueueLimit:    org.AutoAssignQueueLimit,
	}
}

type CreateOrganizationBodyDto struct {
	Name                    string  `json:"name"`
	DefaultScenarioTimezone *string `json:"default_scenario_timezone"`
}

type UpdateOrganizationBodyDto struct {
	DefaultScenarioTimezone *string `json:"default_scenario_timezone,omitempty"`
	SanctionsThreshold      *int    `json:"sanctions_threshold,omitempty"`
	SanctionsLimit          *int    `json:"sanctions_limit,omitempty"`
	AutoAssignQueueLimit    *int    `json:"auto_assign_queue_limit,omitempty"`
}
