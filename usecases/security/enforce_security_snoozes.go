package security

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"

	"github.com/cockroachdb/errors"
)

func (e *EnforceSecurityImpl) ReadSnoozesOfDecision(ctx context.Context, decision models.Decision) error {
	return errors.Join(
		e.Permission(models.READ_SNOOZES),
		utils.EnforceOrganizationAccess(e.Credentials, decision.OrganizationId),
	)
}

func (e *EnforceSecurityImpl) CreateSnoozesOnDecision(ctx context.Context, decision models.Decision) error {
	return errors.Join(
		e.Permission(models.CREATE_SNOOZE),
		utils.EnforceOrganizationAccess(e.Credentials, decision.OrganizationId),
	)
}

func (e *EnforceSecurityImpl) ReadSnoozesOfIteration(ctx context.Context, iteration models.ScenarioIteration) error {
	orgId, _ := uuid.Parse(iteration.OrganizationId)
	return errors.Join(
		e.Permission(models.READ_SNOOZES),
		utils.EnforceOrganizationAccess(e.Credentials, orgId),
	)
}

func (e *EnforceSecurityImpl) ReadRuleSnooze(ctx context.Context, snooze models.RuleSnooze) error {
	orgId, _ := uuid.Parse(snooze.OrganizationId)
	return errors.Join(
		e.Permission(models.READ_SNOOZES),
		utils.EnforceOrganizationAccess(e.Credentials, orgId),
	)
}
