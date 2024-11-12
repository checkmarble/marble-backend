package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityPhantomDecision interface {
	EnforceSecurity
	CreatePhantomDecision(organizationId string) error
}

type EnforceSecurityPhantomDecisionImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityPhantomDecisionImpl) CreatePhantomDecision(organizationId string) error {
	return errors.Join(
		e.Permission(models.PHANTOM_DECISION_CREATE),
		e.ReadOrganization(organizationId),
	)
}
