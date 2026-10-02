package security

import (
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/stretchr/testify/assert"
)

func customRole(slug string, permissions ...models.Permission) models.RoleBinding {
	return models.RoleBinding{Role: models.Role(slug), Permissions: permissions}
}

func credentialsWithRole(role models.Role) models.Credentials {
	return models.Credentials{
		OrganizationId: utils.TextToUUID("org"),
		ActorIdentity:  models.Identity{UserId: "principal"},
		RoleBindings:   models.NativeRoleBindings([]models.Role{role}),
	}
}

func userSecurity(role models.Role) EnforceSecurityUserImpl {
	credentials := credentialsWithRole(role)
	return EnforceSecurityUserImpl{EnforceSecurity: NewEnforceSecurity(credentials), Credentials: credentials}
}

func TestCreateUserCustomRoleGrants(t *testing.T) {
	tts := []struct {
		name      string
		principal models.Role
		bindings  []models.RoleBinding
		allowed   bool
	}{
		{"admin can grant a custom role with permissions it holds", models.ADMIN,
			[]models.RoleBinding{customRole("org/editor", models.DATA_MODEL_WRITE, models.CASE_READ_WRITE)}, true},
		{"admin cannot grant a custom role with a permission it lacks", models.ADMIN,
			[]models.RoleBinding{customRole("org/phantom", models.PHANTOM_DECISION_CREATE)}, false},
		{"native roles are not subject to the rule", models.ADMIN,
			models.NativeRoleBindings([]models.Role{models.PUBLISHER}), true},
		{"marble admin cannot grant platform permissions through a custom role", models.MARBLE_ADMIN,
			[]models.RoleBinding{customRole("org/escalate", models.ANY_ORGANIZATION_ID_IN_CONTEXT)}, false},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			e := userSecurity(tt.principal)

			outcome := e.CreateUser(models.CreateUser{OrganizationId: utils.TextToUUID("org"), RoleBindings: tt.bindings})

			if tt.allowed {
				assert.NoError(t, outcome)
			} else {
				assert.ErrorIs(t, outcome, models.ForbiddenError)
			}
		})
	}
}

func TestUpdateUserCustomRoleGrants(t *testing.T) {
	phantom := customRole("org/phantom", models.PHANTOM_DECISION_CREATE)
	later := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	phantomUntilLater := phantom
	phantomUntilLater.Conditions = models.RoleBindingConditions{NotAfter: &later}

	current := []models.RoleBinding{models.NewNativeRoleBinding(models.VIEWER), phantom}

	tts := []struct {
		name    string
		to      []models.RoleBinding
		allowed bool
	}{
		{"keeping a custom role the admin could not grant", []models.RoleBinding{
			models.NewNativeRoleBinding(models.BUILDER), phantom,
		}, true},
		{"removing a custom role the admin could not grant", []models.RoleBinding{
			models.NewNativeRoleBinding(models.VIEWER),
		}, true},
		{"changing the conditions of a custom role the admin could not grant", []models.RoleBinding{
			models.NewNativeRoleBinding(models.VIEWER), phantomUntilLater,
		}, false},
		{"granting a custom role the admin holds", []models.RoleBinding{
			models.NewNativeRoleBinding(models.VIEWER), phantom, customRole("org/editor", models.DATA_MODEL_WRITE),
		}, true},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			e := userSecurity(models.ADMIN)
			target := models.User{OrganizationId: utils.TextToUUID("org"), UserId: "target", RoleBindings: current}
			bindings := tt.to

			outcome := e.UpdateUser(target, models.UpdateUser{UserId: string(target.UserId), RoleBindings: &bindings})

			if tt.allowed {
				assert.NoError(t, outcome)
			} else {
				assert.ErrorIs(t, outcome, models.ForbiddenError)
			}
		})
	}
}

func TestApiKeyCustomRoleGrants(t *testing.T) {
	credentials := credentialsWithRole(models.ADMIN)
	e := EnforceSecurityApiKeyImpl{EnforceSecurity: NewEnforceSecurity(credentials), Credentials: credentials}

	assert.NoError(t, e.GrantRoleBindings(models.NativeRoleBindings([]models.Role{models.API_CLIENT})),
		"native API_CLIENT holds a permission ADMIN lacks, but native roles are not subject to the rule")
	assert.NoError(t, e.GrantRoleBindings([]models.RoleBinding{customRole("org/reader", models.DECISION_READ)}))
	assert.ErrorIs(t, e.GrantRoleBindings([]models.RoleBinding{customRole("org/phantom", models.PHANTOM_DECISION_CREATE)}),
		models.ForbiddenError)
}

func TestGrantOrganizationRoleBindings(t *testing.T) {
	phantom := customRole("org/phantom", models.PHANTOM_DECISION_CREATE)

	tts := []struct {
		name    string
		current []models.RoleBinding
		next    []models.RoleBinding
		allowed bool
	}{
		{"admin can grant a custom role with permissions it holds", nil,
			[]models.RoleBinding{customRole("org/editor", models.DATA_MODEL_WRITE, models.CASE_READ_WRITE)}, true},
		{"admin cannot grant a custom role with a permission it lacks", nil,
			[]models.RoleBinding{phantom}, false},
		{"admin can keep a custom role granted by someone else", []models.RoleBinding{phantom},
			[]models.RoleBinding{phantom, customRole("org/editor", models.CASE_READ_WRITE)}, true},
		{"admin can remove a custom role it could not grant", []models.RoleBinding{phantom}, nil, true},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			e := userSecurity(models.ADMIN)

			outcome := e.GrantOrganizationRoleBindings(tt.current, tt.next)

			if tt.allowed {
				assert.NoError(t, outcome)
			} else {
				assert.ErrorIs(t, outcome, models.ForbiddenError)
			}
		})
	}
}
