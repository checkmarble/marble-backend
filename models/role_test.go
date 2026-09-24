package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemRoleIsInternal(t *testing.T) {
	assert.Equal(t, Role("SYSTEM"), SYSTEM)
	assert.NotContains(t, GetValidUserRoles(), SYSTEM)
}

func TestSystemRoleHasWorkerPermissionsOnly(t *testing.T) {
	for _, permission := range []Permission{
		ANY_ORGANIZATION_ID_IN_CONTEXT,
		CASE_READ_WRITE,
		DATA_MODEL_READ,
		DECISION_CREATE,
		DECISION_READ,
		INBOX_EDITOR,
		INGESTION,
		PHANTOM_DECISION_CREATE,
		SCENARIO_READ,
		WEBHOOK_EVENT,
	} {
		assert.True(t, SYSTEM.HasPermission(permission), permission)
	}

	assert.False(t, SYSTEM.HasPermission(ORGANIZATIONS_CREATE))
}

func TestLegacyRoleValue(t *testing.T) {
	tests := []struct {
		role     Role
		expected int
	}{
		{VIEWER, 1},
		{BUILDER, 2},
		{PUBLISHER, 3},
		{ADMIN, 4},
		{API_CLIENT, 5},
		{MARBLE_ADMIN, 6},
		{ANALYST, 9},
	}

	for _, test := range tests {
		t.Run(string(test.role), func(t *testing.T) {
			assert.Equal(t, test.expected, LegacyRoleValue([]RoleBinding{NewNativeRoleBinding(test.role)}))
		})
	}
	assert.Zero(t, LegacyRoleValue(nil))
}

func TestCustomRoleSlugValidation(t *testing.T) {
	tests := []struct {
		slug  Role
		valid bool
	}{
		{slug: "org/custom.name", valid: true},
		{slug: "custom.name", valid: false},
		{slug: "org/", valid: false},
		{slug: "org/custom/name", valid: false},
		{slug: "org/custom-name", valid: false},
	}

	for _, test := range tests {
		t.Run(string(test.slug), func(t *testing.T) {
			assert.Equal(t, test.valid, test.slug.IsValidCustom())
		})
	}
}
