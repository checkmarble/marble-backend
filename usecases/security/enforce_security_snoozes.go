package security

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
)

func (e *EnforceSecurityImpl) ReadSnoozesOfDecision(ctx context.Context, decision models.Decision) error {
	return errors.Join(
		e.Permission(models.READ_SNOOZES),
		utils.EnforceOrganizationAccess(e.Credentials, decision.OrganizationId),
	)
}
