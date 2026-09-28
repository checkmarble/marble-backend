package security

import (
	"slices"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockUserEnforceSecurity struct{}

func (mockUserEnforceSecurity) Permission(permission models.Permission) error {
	return nil
}

func (mockUserEnforceSecurity) ReadOrganization(organizationId uuid.UUID) error {
	return nil
}

func (mockUserEnforceSecurity) Permissions(permissions []models.Permission) error {
	return nil
}

func (mockUserEnforceSecurity) UserId() *string {
	return nil
}

func (mockUserEnforceSecurity) ApiKeyId() *string {
	return nil
}

func (mockUserEnforceSecurity) OrgId() uuid.UUID {
	return uuid.Nil
}

func TestUpdateUserRole(t *testing.T) {
	tts := []struct {
		name      string
		sameUser  bool
		principal models.Role
		from, to  models.Role
		allowed   bool
	}{
		{"non-admin can update self without changing role", true, models.VIEWER, models.VIEWER, models.VIEWER, true},
		{"admin can update self without changing role", true, models.ADMIN, models.ADMIN, models.ADMIN, true},
		{"admin cannot drop self admin", true, models.ADMIN, models.ADMIN, models.VIEWER, false},
		{"non-admin cannot change self-role", true, models.VIEWER, models.VIEWER, models.PUBLISHER, false},
		{"non-admin cannot change other's role", false, models.PUBLISHER, models.VIEWER, models.PUBLISHER, false},
		{"non-admin cannot change other's role to admin", false, models.BUILDER, models.VIEWER, models.ADMIN, false},
		{"admin can change other's role", false, models.ADMIN, models.VIEWER, models.PUBLISHER, true},
		{"admin can change other's role to admin", false, models.ADMIN, models.VIEWER, models.ADMIN, true},
		{"admin can change other's admin role", false, models.ADMIN, models.ADMIN, models.VIEWER, true},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			e := EnforceSecurityUserImpl{
				EnforceSecurity: mockUserEnforceSecurity{},
				Credentials: models.Credentials{
					OrganizationId: utils.TextToUUID("org"),
					ActorIdentity:  models.Identity{UserId: "principal"},
					RoleBindings:   models.NativeRoleBindings([]models.Role{tt.principal}),
				},
			}

			target := models.User{OrganizationId: utils.TextToUUID("org"), UserId: "target", RoleBindings: models.NativeRoleBindings([]models.Role{tt.from})}
			if tt.sameUser {
				target.UserId = "principal"
				target.RoleBindings = models.NativeRoleBindings([]models.Role{tt.principal})
			}

			bindings := models.NativeRoleBindings([]models.Role{tt.to})
			update := models.UpdateUser{UserId: string(target.UserId), RoleBindings: &bindings}
			if slices.Equal([]models.Role{tt.principal}, models.RoleNames(bindings)) {
				update.RoleBindings = nil
			}

			outcome := e.UpdateUser(target, update)

			if tt.allowed {
				assert.NoError(t, outcome)
			} else {
				assert.Error(t, outcome)
			}
		})
	}
}

func TestTenantUserAccessRequiresAdmin(t *testing.T) {
	organizationID := uuid.New()
	tests := []struct {
		name    string
		role    models.Role
		allowed bool
	}{
		{name: "viewer", role: models.VIEWER},
		{name: "admin", role: models.ADMIN, allowed: true},
		{name: "marble admin", role: models.MARBLE_ADMIN, allowed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := EnforceSecurityUserImpl{
				EnforceSecurity: mockUserEnforceSecurity{},
				Credentials:     models.Credentials{RoleBindings: models.NativeRoleBindings([]models.Role{tt.role})},
			}

			for _, operation := range []error{
				enforcer.ListTenantUsers(organizationID),
				enforcer.ManageOrganizationGrant(organizationID, models.User{}),
			} {
				if tt.allowed {
					assert.NoError(t, operation)
				} else {
					assert.ErrorIs(t, operation, models.ForbiddenError)
				}
			}
		})
	}
}

func TestManageOrganizationGrantForMarbleAdmin(t *testing.T) {
	organizationID := uuid.New()
	target := models.User{RoleBindings: models.NativeRoleBindings([]models.Role{models.MARBLE_ADMIN})}

	for _, tt := range []struct {
		name    string
		role    models.Role
		allowed bool
	}{
		{name: "organization admin", role: models.ADMIN},
		{name: "marble admin", role: models.MARBLE_ADMIN, allowed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := EnforceSecurityUserImpl{
				EnforceSecurity: mockUserEnforceSecurity{},
				Credentials:     models.Credentials{RoleBindings: models.NativeRoleBindings([]models.Role{tt.role})},
			}
			err := enforcer.ManageOrganizationGrant(organizationID, target)
			if tt.allowed {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, models.ForbiddenError)
		})
	}
}

func TestUpdateUserMarbleAdminRole(t *testing.T) {
	roles := func(roles ...models.Role) []models.RoleBinding { return models.NativeRoleBindings(roles) }

	tts := []struct {
		name      string
		principal []models.Role
		from, to  []models.Role
		allowed   bool
	}{
		{"admin cannot revoke marble admin", []models.Role{models.ADMIN},
			[]models.Role{models.ADMIN, models.MARBLE_ADMIN}, []models.Role{models.ADMIN}, false},
		{"admin cannot grant marble admin", []models.Role{models.ADMIN},
			[]models.Role{models.VIEWER}, []models.Role{models.VIEWER, models.MARBLE_ADMIN}, false},
		{"admin can change other roles of a marble admin", []models.Role{models.ADMIN},
			[]models.Role{models.VIEWER, models.MARBLE_ADMIN}, []models.Role{models.PUBLISHER, models.MARBLE_ADMIN}, true},
		{"marble admin can revoke marble admin", []models.Role{models.MARBLE_ADMIN},
			[]models.Role{models.ADMIN, models.MARBLE_ADMIN}, []models.Role{models.ADMIN}, true},
		{"marble admin can grant marble admin", []models.Role{models.MARBLE_ADMIN},
			[]models.Role{models.VIEWER}, []models.Role{models.VIEWER, models.MARBLE_ADMIN}, true},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			e := EnforceSecurityUserImpl{
				EnforceSecurity: mockUserEnforceSecurity{},
				Credentials: models.Credentials{
					OrganizationId: utils.TextToUUID("org"),
					ActorIdentity:  models.Identity{UserId: "principal"},
					RoleBindings:   roles(tt.principal...),
				},
			}

			target := models.User{OrganizationId: utils.TextToUUID("org"), UserId: "target", RoleBindings: roles(tt.from...)}
			bindings := roles(tt.to...)

			outcome := e.UpdateUser(target, models.UpdateUser{UserId: string(target.UserId), RoleBindings: &bindings})

			if tt.allowed {
				assert.NoError(t, outcome)
			} else {
				assert.ErrorIs(t, outcome, models.BadParameterError)
			}
		})
	}
}

func TestUpdateUserMarbleAdminConditions(t *testing.T) {
	past := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	required := true

	marbleAdmin := func(conditions models.RoleBindingConditions) models.RoleBinding {
		binding := models.NewNativeRoleBinding(models.MARBLE_ADMIN)
		binding.Conditions = conditions
		return binding
	}
	current := []models.RoleBinding{
		models.NewNativeRoleBinding(models.ADMIN),
		marbleAdmin(models.RoleBindingConditions{UsedSecondFactor: &required}),
	}

	tts := []struct {
		name      string
		principal models.Role
		to        []models.RoleBinding
		allowed   bool
	}{
		{"admin can keep the marble admin binding unchanged", models.ADMIN, current, true},
		{"admin cannot expire the marble admin binding", models.ADMIN, []models.RoleBinding{
			models.NewNativeRoleBinding(models.ADMIN),
			marbleAdmin(models.RoleBindingConditions{UsedSecondFactor: &required, NotAfter: &past}),
		}, false},
		{"admin cannot remove the marble admin caveats", models.ADMIN, []models.RoleBinding{
			models.NewNativeRoleBinding(models.ADMIN),
			marbleAdmin(models.RoleBindingConditions{}),
		}, false},
		{"marble admin can change the marble admin caveats", models.MARBLE_ADMIN, []models.RoleBinding{
			models.NewNativeRoleBinding(models.ADMIN),
			marbleAdmin(models.RoleBindingConditions{}),
		}, true},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			e := EnforceSecurityUserImpl{
				EnforceSecurity: mockUserEnforceSecurity{},
				Credentials: models.Credentials{
					OrganizationId: utils.TextToUUID("org"),
					ActorIdentity:  models.Identity{UserId: "principal"},
					RoleBindings:   models.NativeRoleBindings([]models.Role{tt.principal}),
				},
			}

			target := models.User{OrganizationId: utils.TextToUUID("org"), UserId: "target", RoleBindings: current}
			bindings := tt.to

			outcome := e.UpdateUser(target, models.UpdateUser{UserId: string(target.UserId), RoleBindings: &bindings})

			if tt.allowed {
				assert.NoError(t, outcome)
			} else {
				assert.ErrorIs(t, outcome, models.BadParameterError)
			}
		})
	}
}
