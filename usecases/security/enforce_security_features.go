package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecurityFeatures interface {
	EnforceSecurity
	ReadFeature(feature models.Feature) error
	CreateFeature() error
	UpdateFeature(feature models.Feature) error
	DeleteFeature(feature models.Feature) error
}

func (e *EnforceSecurityImpl) ReadFeature(feature models.Feature) error {
	return errors.Join(
		e.Permission(models.FEATURE_READ),
	)
}

func (e *EnforceSecurityImpl) CreateFeature() error {
	return errors.Join(
		e.Permission(models.FEATURE_CREATE),
	)
}

func (e *EnforceSecurityImpl) UpdateFeature(feature models.Feature) error {
	return errors.Join(
		e.Permission(models.FEATURE_UPDATE),
	)
}

func (e *EnforceSecurityImpl) DeleteFeature(feature models.Feature) error {
	return errors.Join(
		e.Permission(models.FEATURE_DELETE),
	)
}
