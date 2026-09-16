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
