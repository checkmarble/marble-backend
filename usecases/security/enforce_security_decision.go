package security

import (
	"errors"
	"marble/marble-backend/models"
)

type EnforceSecurityDecision interface {
	EnforceSecurity
	ReadDecision(decision models.Decision) error
	CreateDecision(organizationId string) error
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
