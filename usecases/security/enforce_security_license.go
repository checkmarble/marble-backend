package security

import (
	"context"
	"errors"

	"github.com/checkmarble/marble-backend/models"
)

func (e *EnforceSecurityImpl) ListLicenses(ctx context.Context) error {
	return errors.Join(
		e.Permission(models.LICENSE_LIST),
	)
}

func (e *EnforceSecurityImpl) CreateLicense(ctx context.Context) error {
	return errors.Join(
		e.Permission(models.LICENSE_CREATE),
	)
}

func (e *EnforceSecurityImpl) UpdateLicense(ctx context.Context) error {
	return errors.Join(
		e.Permission(models.LICENSE_UPDATE),
	)
}
