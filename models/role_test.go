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
