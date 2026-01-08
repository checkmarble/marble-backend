package security

import (
	"testing"

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
					Role:           tt.principal,
				},
			}

			target := models.User{OrganizationId: utils.TextToUUID("org"), UserId: "target", Role: tt.from}
			if tt.sameUser {
				target.UserId = "principal"
				target.Role = tt.principal
			}

			update := models.UpdateUser{UserId: string(target.UserId), Role: &tt.to}
			if tt.principal == *update.Role {
				update.Role = nil
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
