package security

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEnforceSecurityCaseForUserUsesOrganizationGrant(t *testing.T) {
	org, inbox := uuid.New(), uuid.New()
	user := models.User{OrganizationRoles: map[uuid.UUID][]models.Role{org: {models.VIEWER}}}
	caseMetadata := models.CaseMetadata{OrganizationId: org, InboxId: inbox}
	require.NoError(t, EnforceSecurityCaseForUser(user, org).ReadOrUpdateCase(caseMetadata, []uuid.UUID{inbox}))
	delete(user.OrganizationRoles, org)
	require.ErrorIs(t, EnforceSecurityCaseForUser(user, org).ReadOrUpdateCase(caseMetadata, []uuid.UUID{inbox}), models.ForbiddenError)
}
