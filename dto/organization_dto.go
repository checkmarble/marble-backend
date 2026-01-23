package dto

import (
	"encoding/json"
	"net"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
)

type APIOrganization struct {
	Id                      string      `json:"id"`
	Name                    string      `json:"name"`
	DefaultScenarioTimezone *string     `json:"default_scenario_timezone"`
	SanctionsThreshold      int         `json:"sanctions_threshold"`
	SanctionsLimit          int         `json:"sanctions_limit"`
	AutoAssignQueueLimit    int         `json:"auto_assign_queue_limit"`
	AllowedNetworks         []SubnetDto `json:"allowed_networks"`
	SentryReplayEnabled     bool        `json:"sentry_replay_enabled"`
	Environment             string      `json:"environment"`
}

func AdaptOrganizationDto(org models.Organization) APIOrganization {
	return APIOrganization{
		Id:                      org.Id.String(),
		Name:                    org.Name,
		DefaultScenarioTimezone: org.DefaultScenarioTimezone,
		SanctionsThreshold:      org.OpenSanctionsConfig.MatchThreshold,
		SanctionsLimit:          org.OpenSanctionsConfig.MatchLimit,
		AutoAssignQueueLimit:    org.AutoAssignQueueLimit,
		AllowedNetworks: pure_utils.Map(org.WhitelistedSubnets, func(subnet net.IPNet) SubnetDto {
			return SubnetDto{subnet}
		}),
		SentryReplayEnabled: org.SentryReplayEnabled,
		Environment:         org.Environment.String(),
	}
}

type CreateOrganizationBodyDto struct {
	Name                    string  `json:"name"`
	DefaultScenarioTimezone *string `json:"default_scenario_timezone"`
	Environment             *string `json:"environment"`
}

func AdaptCreateOrganizationInput(dto CreateOrganizationBodyDto) (models.CreateOrganizationInput, error) {
	var env *models.OrganizationEnvironment
	if dto.Environment != nil {
		parsedEnv := models.ParseOrganizationEnvironment(*dto.Environment)
		if parsedEnv == models.OrganizationEnvironmentUnknown {
			return models.CreateOrganizationInput{}, errors.Wrapf(models.BadParameterError,
				"Invalid organization environment %s", *dto.Environment)
		}
		env = &parsedEnv
	}
	return models.CreateOrganizationInput{
		Name:        dto.Name,
		Environment: env,
	}, nil
}

type UpdateOrganizationBodyDto struct {
	DefaultScenarioTimezone *string `json:"default_scenario_timezone,omitempty"`
	SanctionsThreshold      *int    `json:"sanctions_threshold,omitempty"`
	SanctionsLimit          *int    `json:"sanctions_limit,omitempty"`
	AutoAssignQueueLimit    *int    `json:"auto_assign_queue_limit,omitempty"`
	SentryReplayEnabled     *bool   `json:"sentry_replay_enabled"`
	Environment             *string `json:"environment"`
}

func AdaptUpdateOrganizationInput(dto UpdateOrganizationBodyDto) (models.UpdateOrganizationInput, error) {
	out := models.UpdateOrganizationInput{
		DefaultScenarioTimezone: dto.DefaultScenarioTimezone,
		ScreeningConfig: models.OrganizationOpenSanctionsConfigUpdateInput{
			MatchThreshold: dto.SanctionsThreshold,
			MatchLimit:     dto.SanctionsLimit,
		},
		AutoAssignQueueLimit: dto.AutoAssignQueueLimit,
		SentryReplayEnabled:  dto.SentryReplayEnabled,
	}

	if dto.Environment != nil {
		parsedEnv := models.ParseOrganizationEnvironment(*dto.Environment)
		if parsedEnv == models.OrganizationEnvironmentUnknown {
			return models.UpdateOrganizationInput{}, errors.Wrapf(models.BadParameterError,
				"Invalid organization environment %s", *dto.Environment)
		}
		out.Environment = &parsedEnv
	}
	return out, nil
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

	// If a bare IP address was given
	if !strings.Contains(cidr, "/") {
		ip := net.ParseIP(cidr)

		if ip == nil {
			return errors.Newf("invalid CIDR-less IP address: %s", cidr)
		}

		switch {
		case ip.To4() != nil:
			cidr = ip.String() + "/32"
		case ip.To16() != nil:
			cidr = ip.String() + "/128"
		default:
			return errors.Newf("invalid CIDR-less IP address %s", cidr)
		}
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
