package dto

import (
	"encoding/json"
	"net"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type APIOrganization struct {
	Id                      string      `json:"id"`
	Name                    string      `json:"name"`
	DefaultScenarioTimezone *string     `json:"default_scenario_timezone"`
	SanctionsThreshold      int         `json:"sanctions_threshold"`
	SanctionsLimit          int         `json:"sanctions_limit"`
	AutoAssignQueueLimit    int         `json:"auto_assign_queue_limit"`
	AllowedNetworks         []SubnetDto `json:"allowed_networks"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		Id:                      org.Id,
		Name:                    org.Name,
		DefaultScenarioTimezone: org.DefaultScenarioTimezone,
		SanctionsThreshold:      org.OpenSanctionsConfig.MatchThreshold,
		SanctionsLimit:          org.OpenSanctionsConfig.MatchLimit,
		AutoAssignQueueLimit:    org.AutoAssignQueueLimit,
		AllowedNetworks: pure_utils.Map(org.WhitelistedSubnets, func(subnet net.IPNet) SubnetDto {
			return SubnetDto{subnet}
		}),
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

type OrganizationSubnetsDto struct {
	Subnets []SubnetDto `json:"subnets"`
}

func AdaptOrganizationSubnet(dto net.IPNet) SubnetDto {
	return SubnetDto{dto}
}

type SubnetDto struct {
	net.IPNet
}

func (s *SubnetDto) UnmarshalJSON(b []byte) error {
	var cidr string

	if err := json.Unmarshal(b, &cidr); err != nil {
		return err
	}

	_, subnet, err := net.ParseCIDR(cidr)

	if err != nil {
		return err
	}

	*s = SubnetDto{*subnet}

	return nil
}

func (s *SubnetDto) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}
