package models

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlatformPermissionsCannotBeGrantedThroughCustomRoles(t *testing.T) {
	for _, permission := range PLATFORM_PERMISSIONS {
		assert.NotContains(t, ValidPermissions, permission)
	}
}

func TestOrganizationRolesHoldNoPlatformPermission(t *testing.T) {
	for _, role := range []Role{VIEWER, ANALYST, BUILDER, PUBLISHER, ADMIN, API_CLIENT} {
		assert.False(t, slices.ContainsFunc(role.Permissions(), Permission.IsPlatform), role)
	}
}

func TestWithoutPlatformPermissions(t *testing.T) {
	permissions := []Permission{DECISION_READ, ANY_ORGANIZATION_ID_IN_CONTEXT, CASE_READ_WRITE, LICENSE_CREATE}

	assert.Equal(t, []Permission{DECISION_READ, CASE_READ_WRITE}, WithoutPlatformPermissions(permissions))
	assert.Len(t, permissions, 4, "the input must not be modified")
}
