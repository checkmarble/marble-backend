package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type EnforceSecurityContinuousScreening interface {
	EnforceSecurity
	ReadContinuousScreeningConfig(config models.ContinuousScreeningConfig) error
	WriteContinuousScreeningConfig(orgId uuid.UUID) error
	ReadContinuousScreeningObject(orgId uuid.UUID) error
	WriteContinuousScreeningObject(orgId uuid.UUID) error
	ReadContinuousScreeningHit(hit models.ContinuousScreeningWithMatches) error
	WriteContinuousScreeningHit(orgId uuid.UUID) error
	DismissContinuousScreeningHits(orgId uuid.UUID) error
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

func (e *EnforceSecurityContinuousScreeningImpl) WriteContinuousScreeningConfig(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_CONFIG_WRITE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityContinuousScreeningImpl) ReadContinuousScreeningObject(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_OBJECT_READ),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityContinuousScreeningImpl) WriteContinuousScreeningObject(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_OBJECT_WRITE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityContinuousScreeningImpl) ReadContinuousScreeningHit(hit models.ContinuousScreeningWithMatches) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_HIT_READ),
		e.ReadOrganization(hit.OrgId),
	)
}

func (e *EnforceSecurityContinuousScreeningImpl) WriteContinuousScreeningHit(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_HIT_WRITE),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityContinuousScreeningImpl) DismissContinuousScreeningHits(orgId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.CONTINUOUS_SCREENING_HIT_DISMISS),
		e.ReadOrganization(orgId),
	)
}
