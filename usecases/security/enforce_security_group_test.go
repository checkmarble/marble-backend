package security

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func groupSecurity(bindings ...models.RoleBinding) EnforceSecurityGroupImpl {
	credentials := models.Credentials{
		OrganizationId: utils.TextToUUID("org"),
		ActorIdentity:  models.Identity{UserId: "principal"},
		RoleBindings:   bindings,
	}

	return EnforceSecurityGroupImpl{EnforceSecurity: NewEnforceSecurity(credentials), Credentials: credentials}
}

func groupBinding(role models.Role, groupId uuid.UUID) models.RoleBinding {
	binding := models.NewNativeRoleBinding(role)
	binding.GroupId = &groupId

	return binding
}

func TestGrantGroupRoleBindings(t *testing.T) {
	manager := customRole("org/manager", models.MANAGE_ROLES, models.CASE_READ_WRITE)
	viewer := models.NativeRoleBindings([]models.Role{models.VIEWER})

	tts := []struct {
		name     string
		actor    models.RoleBinding
		current  []models.RoleBinding
		next     []models.RoleBinding
		expected error
	}{
		{"admin can grant a native role", models.NewNativeRoleBinding(models.ADMIN),
			nil, viewer, nil},
		{"non-admin cannot grant a native role", manager,
			nil, viewer, models.ForbiddenError},
		{"non-admin can keep an unchanged native role", manager,
			viewer, viewer, nil},
		{"non-admin can grant a custom role with permissions it holds", manager,
			nil, []models.RoleBinding{customRole("org/cases", models.CASE_READ_WRITE)}, nil},
		{"non-admin cannot grant a custom role with a permission it lacks", manager,
			nil, []models.RoleBinding{customRole("org/editor", models.DATA_MODEL_WRITE)}, models.ForbiddenError},
		{"platform permissions cannot be granted through a custom role", models.NewNativeRoleBinding(models.MARBLE_ADMIN),
			nil, []models.RoleBinding{customRole("org/escalate", models.ANY_ORGANIZATION_ID_IN_CONTEXT)}, models.ForbiddenError},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			e := groupSecurity(tt.actor)

			err := e.GrantGroupRoleBindings(tt.current, tt.next)

			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tt.expected)
			}
		})
	}
}

func TestKeepOwnAdmin(t *testing.T) {
	groupId := utils.TextToUUID("group")
	otherGroupId := utils.TextToUUID("other-group")

	tts := []struct {
		name     string
		bindings []models.RoleBinding
		allowed  bool
	}{
		{"admin only through the group", []models.RoleBinding{groupBinding(models.ADMIN, groupId)}, false},
		{"admin directly as well", []models.RoleBinding{
			groupBinding(models.ADMIN, groupId), models.NewNativeRoleBinding(models.ADMIN),
		}, true},
		{"admin through another group", []models.RoleBinding{
			groupBinding(models.ADMIN, groupId), groupBinding(models.ADMIN, otherGroupId),
		}, true},
		{"not an admin", []models.RoleBinding{groupBinding(models.VIEWER, groupId)}, true},
		{"marble admin", []models.RoleBinding{
			groupBinding(models.ADMIN, groupId), models.NewNativeRoleBinding(models.MARBLE_ADMIN),
		}, true},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			e := groupSecurity(tt.bindings...)

			err := e.KeepOwnAdmin(groupId)

			if tt.allowed {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, models.BadParameterError)
			}
		})
	}
}
