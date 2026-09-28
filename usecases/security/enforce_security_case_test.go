package security

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEnforceSecurityCaseForUserIgnoresRequestCaveats(t *testing.T) {
	orgId := utils.TextToUUID("org")
	inboxId := utils.TextToUUID("inbox")
	required := true

	assignee := models.User{
		UserId:         "assignee",
		OrganizationId: orgId,
		RoleBindings: []models.RoleBinding{{
			Role:        models.VIEWER,
			OrgId:       orgId,
			Permissions: models.VIEWER.Permissions(),
			Conditions:  models.RoleBindingConditions{UsedSecondFactor: &required},
		}},
	}
	c := models.CaseMetadata{OrganizationId: orgId, InboxId: inboxId}

	// The assignee holds case permissions behind a second factor, which can
	// only be known from its own requests.
	assert.NoError(t, EnforceSecurityCaseForUser(assignee).ReadOrUpdateCase(c, []uuid.UUID{inboxId}))
}
