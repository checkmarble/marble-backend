package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemRoleIsInternal(t *testing.T) {
	assert.Equal(t, "SYSTEM", SYSTEM.String())
	assert.NotContains(t, GetValidUserRoles(), SYSTEM)
	assert.Equal(t, NO_ROLE, RoleFromString("SYSTEM"))
}

func TestSystemRoleHasWorkerPermissionsOnly(t *testing.T) {
	for _, permission := range []Permission{
		ANY_ORGANIZATION_ID_IN_CONTEXT,
		PHANTOM_DECISION_CREATE,
		WEBHOOK_EVENT,
		CASE_READ_WRITE,
		DECISION_READ,
		DECISION_CREATE,
		SCENARIO_READ,
		INGESTION,
	} {
		assert.True(t, SYSTEM.HasPermission(permission), permission)
	}

	assert.False(t, SYSTEM.HasPermission(ORGANIZATIONS_CREATE))
}
