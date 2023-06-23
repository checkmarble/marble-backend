package utils

import (
	"marble/marble-backend/models"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnforceOrganizationAccess(t *testing.T) {
	err := EnforceOrganizationAccess(models.Credentials{
		OrganizationId: "1234",
		Role:           models.NO_ROLE,
	}, "1234")
	assert.NoError(t, err)
}

func TestEnforceOrganizationAccess_EmptyCredential(t *testing.T) {
	err := EnforceOrganizationAccess(models.Credentials{}, "1234")
	assert.ErrorIs(t, err, models.ForbiddenError)
}

func TestEnforceOrganizationAccess_Fail(t *testing.T) {
	err := EnforceOrganizationAccess(models.Credentials{OrganizationId: "not 1234"}, "1234")
	assert.ErrorIs(t, err, models.ForbiddenError)
}

func TestEnforceOrganizationAccess_marble_admin_override(t *testing.T) {
	err := EnforceOrganizationAccess(models.Credentials{Role: models.MARBLE_ADMIN}, "1234")
	assert.NoError(t, err)
}
