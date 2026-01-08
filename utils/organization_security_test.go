package utils

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"

	"github.com/stretchr/testify/assert"
)

func TestEnforceOrganizationAccess(t *testing.T) {
	orgId := TextToUUID("1234")
	err := EnforceOrganizationAccess(models.Credentials{
		OrganizationId: orgId,
		Role:           models.NO_ROLE,
	}, orgId)
	assert.NoError(t, err)
}

func TestEnforceOrganizationAccess_EmptyCredential(t *testing.T) {
	orgId := TextToUUID("1234")
	err := EnforceOrganizationAccess(models.Credentials{}, orgId)
	assert.ErrorIs(t, err, models.ForbiddenError)
}

func TestEnforceOrganizationAccess_Fail(t *testing.T) {
	orgId1 := TextToUUID("not 1234")
	orgId2 := TextToUUID("1234")
	err := EnforceOrganizationAccess(models.Credentials{OrganizationId: orgId1}, orgId2)
	assert.ErrorIs(t, err, models.ForbiddenError)
}

func TestEnforceOrganizationAccess_marble_admin_override(t *testing.T) {
	orgId := TextToUUID("1234")
	err := EnforceOrganizationAccess(models.Credentials{Role: models.MARBLE_ADMIN}, orgId)
	assert.NoError(t, err)
}
