package security

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
)

func TestEditScreeningProvider(t *testing.T) {
	orgId := utils.TextToUUID("org")

	tts := []struct {
		name            string
		role            models.Role
		isManagedMarble bool
		allowed         bool
	}{
		{"managed: marble admin can change provider", models.MARBLE_ADMIN, true, true},
		{"managed: org admin cannot change provider", models.ADMIN, true, false},
		{"managed: viewer cannot change provider", models.VIEWER, true, false},
		{"self-hosted: org admin can change provider", models.ADMIN, false, true},
		{"self-hosted: marble admin can change provider", models.MARBLE_ADMIN, false, true},
		{"self-hosted: viewer cannot change provider (no ORGANIZATIONS_UPDATE)", models.VIEWER, false, false},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			creds := models.Credentials{
				OrganizationId: orgId,
				ActorIdentity:  models.Identity{UserId: "principal"},
				Role:           tt.role,
			}
			e := EnforceSecurityOrganizationImpl{
				EnforceSecurity: &EnforceSecurityImpl{Credentials: creds},
				Credentials:     creds,
			}

			err := e.EditOrganizationScreeningProvider(models.Organization{Id: orgId}, tt.isManagedMarble)

			if tt.allowed {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, models.ForbiddenError),
					"expected a ForbiddenError, got %v", err)
			}
		})
	}
}
