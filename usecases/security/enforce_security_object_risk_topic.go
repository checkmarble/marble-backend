package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityObjectRiskTopic interface {
	EnforceSecurity
	ReadObjectRiskTopic(objectRiskTopic models.ObjectRiskTopic) error
	WriteObjectRiskTopic(orgId uuid.UUID) error
}

type EnforceSecurityObjectRiskTopicImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityObjectRiskTopicImpl) ReadObjectRiskTopic(objectRiskTopic models.ObjectRiskTopic) error {
	return errors.Join(
		e.Permission(models.OBJECT_RISK_TOPIC_READ),
		e.ReadOrganization(objectRiskTopic.OrgId),
	)
}

func (e *EnforceSecurityObjectRiskTopicImpl) WriteObjectRiskTopic(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.OBJECT_RISK_TOPIC_WRITE),
		e.ReadOrganization(orgId),
	)
}
