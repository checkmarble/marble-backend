package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityDecision interface {
	EnforceSecurity
	ReadDecision(decision models.Decision) error
	ReadScheduledExecution(scheduledExecution models.ScheduledExecution) error
	CreateDecision(organizationId string) error
	CreateScheduledExecution(organizationId string) error
}

type EnforceSecurityDecisionImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityDecisionImpl) ReadDecision(decision models.Decision) error {
	return errors.Join(
		e.Permission(models.DECISION_READ),
		e.ReadOrganization(decision.OrganizationId),
	)
}

func (e *EnforceSecurityDecisionImpl) CreateDecision(organizationId string) error {
	return errors.Join(
		e.Permission(models.DECISION_CREATE),
		e.ReadOrganization(organizationId),
	)
}

func (e *EnforceSecurityDecisionImpl) ReadScheduledExecution(scheduledExecution models.ScheduledExecution) error {
	return errors.Join(
		e.Permission(models.DECISION_READ),
		e.ReadOrganization(scheduledExecution.OrganizationId),
	)
}

func (e *EnforceSecurityDecisionImpl) CreateScheduledExecution(organizationId string) error {
	return errors.Join(
		e.Permission(models.DECISION_CREATE),
		e.ReadOrganization(organizationId),
	)
}
