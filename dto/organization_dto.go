package dto

import "github.com/checkmarble/marble-backend/models"

type APIOrganization struct {
	Id                      string   `json:"id"`
	Name                    string   `json:"name"`
	DefaultScenarioTimezone *string  `json:"default_scenario_timezone"`
	SanctionCheckDatasets   []string `json:"sanction_check_datasets"`
	SanctionCheckThreshold  *int     `json:"sanction_check_threshold"`
	SanctionCheckLimit      *int     `json:"sanction_check_limit"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		Id:                      org.Id,
		Name:                    org.Name,
		DefaultScenarioTimezone: org.DefaultScenarioTimezone,
		SanctionCheckDatasets:   org.OpenSanctionsConfig.Datasets,
		SanctionCheckThreshold:  org.OpenSanctionsConfig.MatchThreshold,
		SanctionCheckLimit:      org.OpenSanctionsConfig.MatchLimit,
	}
}

type CreateOrganizationBodyDto struct {
	Name                    string  `json:"name"`
	DefaultScenarioTimezone *string `json:"default_scenario_timezone"`
}

type UpdateOrganizationBodyDto struct {
	DefaultScenarioTimezone *string  `json:"default_scenario_timezone,omitempty"`
	SanctionCheckDatasets   []string `json:"sanction_check_datasets,omitempty"`
	SanctionCheckThreshold  *int     `json:"sanction_check_threshold,omitempty"`
	SanctionCheckLimit      *int     `json:"sanction_check_limit,omitempty"`
}
