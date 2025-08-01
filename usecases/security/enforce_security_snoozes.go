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
		utils.EnforceOrganizationAccess(e.Credentials, decision.OrganizationId.String()),
	)
}

func (e *EnforceSecurityImpl) CreateSnoozesOnDecision(ctx context.Context, decision models.Decision) error {
	return errors.Join(
		e.Permission(models.CREATE_SNOOZE),
		utils.EnforceOrganizationAccess(e.Credentials, decision.OrganizationId.String()),
	)
}

func (e *EnforceSecurityImpl) ReadSnoozesOfIteration(ctx context.Context, iteration models.ScenarioIteration) error {
	return errors.Join(
		e.Permission(models.READ_SNOOZES),
		utils.EnforceOrganizationAccess(e.Credentials, iteration.OrganizationId),
	)
}

func (e *EnforceSecurityImpl) ReadRuleSnooze(ctx context.Context, snooze models.RuleSnooze) error {
	return errors.Join(
		e.Permission(models.READ_SNOOZES),
		utils.EnforceOrganizationAccess(e.Credentials, snooze.OrganizationId),
	)
}
