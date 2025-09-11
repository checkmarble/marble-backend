package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeWithLicenseEntitlement(t *testing.T) {
	tests := []struct {
		name            string
		dbFeatureAccess DbStoredOrganizationFeatureAccess
		license         LicenseEntitlements
		config          FeaturesConfiguration
		user            User
		expected        OrganizationFeatureAccess
	}{
		{
			name: "All features allowed by license and configuration",
			dbFeatureAccess: DbStoredOrganizationFeatureAccess{
				Id:             "1",
				OrganizationId: "org1",
				TestRun:        Allowed,
				Sanctions:      Allowed,
				CaseAutoAssign: Allowed,
			},
			license: LicenseEntitlements{
				Analytics:      true,
				Webhooks:       true,
				Workflows:      true,
				RuleSnoozes:    true,
				UserRoles:      true,
				TestRun:        true,
				Sanctions:      true,
				CaseAutoAssign: true,
			},
			config: FeaturesConfiguration{
				Webhooks:        true,
				Sanctions:       true,
				NameRecognition: true,
				Analytics:       true,
			},
			user: User{AiAssistEnabled: true},
			expected: OrganizationFeatureAccess{
				Id:              "1",
				OrganizationId:  "org1",
				TestRun:         Allowed,
				Sanctions:       Allowed,
				NameRecognition: Allowed,
				Analytics:       Allowed,
				Webhooks:        Allowed,
				Workflows:       Allowed,
				RuleSnoozes:     Allowed,
				Roles:           Allowed,
				AiAssist:        Allowed,
				CaseAutoAssign:  Allowed,
			},
		},
		{
			name: "Some features restricted by license",
			dbFeatureAccess: DbStoredOrganizationFeatureAccess{
				Id:             "2",
				OrganizationId: "org2",
				TestRun:        Allowed,
				Sanctions:      Allowed,
				CaseAutoAssign: Allowed,
			},
			license: LicenseEntitlements{
				Analytics:   false,
				Webhooks:    true,
				Workflows:   true,
				RuleSnoozes: true,
				UserRoles:   true,
				TestRun:     false,
				Sanctions:   false,
			},
			config: FeaturesConfiguration{
				Webhooks:        true,
				Sanctions:       true,
				NameRecognition: true,
				Analytics:       true,
			},
			user: User{AiAssistEnabled: true},
			expected: OrganizationFeatureAccess{
				Id:              "2",
				OrganizationId:  "org2",
				TestRun:         Restricted,
				Sanctions:       Restricted,
				NameRecognition: Restricted,
				Analytics:       Restricted,
				Webhooks:        Allowed,
				Workflows:       Allowed,
				RuleSnoozes:     Allowed,
				Roles:           Allowed,
				AiAssist:        Allowed,
				CaseAutoAssign:  Restricted,
			},
		},
		{
			name: "Some features restricted by configuration",
			dbFeatureAccess: DbStoredOrganizationFeatureAccess{
				Id:             "3",
				OrganizationId: "org3",
				TestRun:        Allowed,
				Sanctions:      Allowed,
				CaseAutoAssign: Allowed,
			},
			license: LicenseEntitlements{
				Analytics:      true,
				Webhooks:       true,
				Workflows:      true,
				RuleSnoozes:    true,
				UserRoles:      true,
				TestRun:        true,
				Sanctions:      true,
				CaseAutoAssign: true,
			},
			config: FeaturesConfiguration{
				Webhooks:        false,
				Sanctions:       false,
				NameRecognition: false,
				Analytics:       false,
			},
			user: User{AiAssistEnabled: false},
			expected: OrganizationFeatureAccess{
				Id:              "3",
				OrganizationId:  "org3",
				TestRun:         Allowed,
				Sanctions:       MissingConfiguration,
				NameRecognition: MissingConfiguration,
				Analytics:       MissingConfiguration,
				Webhooks:        MissingConfiguration,
				Workflows:       Allowed,
				RuleSnoozes:     Allowed,
				Roles:           Allowed,
				AiAssist:        Restricted,
				CaseAutoAssign:  Allowed,
			},
		},
		{
			name: "Test mode enabled",
			dbFeatureAccess: DbStoredOrganizationFeatureAccess{
				Id:             "4",
				OrganizationId: "org4",
				TestRun:        Restricted,
				Sanctions:      Allowed,
				CaseAutoAssign: Allowed,
			},
			license: LicenseEntitlements{
				Analytics:      false,
				Webhooks:       false,
				Workflows:      false,
				RuleSnoozes:    true,
				UserRoles:      true,
				TestRun:        false,
				Sanctions:      true,
				CaseAutoAssign: false,
			},
			config: FeaturesConfiguration{
				Webhooks:        false,
				Sanctions:       true,
				NameRecognition: false,
				Analytics:       false,
			},
			user: User{AiAssistEnabled: false},
			expected: OrganizationFeatureAccess{
				Id:              "4",
				OrganizationId:  "org4",
				TestRun:         Test,
				Sanctions:       Allowed,
				NameRecognition: MissingConfiguration,
				Analytics:       MissingConfiguration,
				Webhooks:        MissingConfiguration,
				Workflows:       Test,
				RuleSnoozes:     Allowed,
				Roles:           Allowed,
				AiAssist:        Restricted,
				CaseAutoAssign:  Test,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dbFeatureAccess.MergeWithLicenseEntitlement(tt.license, tt.config, &tt.user)
			assert.Equal(t, tt.expected, result, tt.name)
		})
	}
}
