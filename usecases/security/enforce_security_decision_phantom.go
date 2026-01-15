package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityPhantomDecision interface {
	EnforceSecurity
	CreatePhantomDecision(organizationId uuid.UUID) error
}

type EnforceSecurityPhantomDecisionImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityPhantomDecisionImpl) CreatePhantomDecision(organizationId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.PHANTOM_DECISION_CREATE),
		e.ReadOrganization(organizationId),
	)
}
