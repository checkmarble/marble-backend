package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

type EnforceSecurityContinuousScreening interface {
	EnforceSecurity
	ReadContinuousScreeningConfig(config models.ContinuousScreeningConfig) error
	WriteContinuousScreeningConfig(orgId string) error
	WriteContinuousScreeningObject(orgId string) error
	ReadContinuousScreeningHit(hit models.ContinuousScreeningWithMatches) error
}

type EnforceSecurityContinuousScreeningImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityContinuousScreeningImpl) ReadContinuousScreeningConfig(config models.ContinuousScreeningConfig) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_CONFIG_READ),
		e.ReadOrganization(config.OrgId),
	)
}

func (e *EnforceSecurityContinuousScreeningImpl) WriteContinuousScreeningConfig(orgId string) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_CONFIG_WRITE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityContinuousScreeningImpl) WriteContinuousScreeningObject(orgId string) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_OBJECT_WRITE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityContinuousScreeningImpl) ReadContinuousScreeningHit(hit models.ContinuousScreeningWithMatches) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_HIT_READ),
		e.ReadOrganization(hit.OrgId.String()),
	)
}
