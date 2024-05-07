package security

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (e *EnforceSecurityImpl) ListPartners(ctx context.Context) error {
	return errors.Join(
		e.Permission(models.PARTNER_LIST),
	)
}

func (e *EnforceSecurityImpl) CreatePartner(ctx context.Context) error {
	return errors.Join(
		e.Permission(models.PARTNER_CREATE),
	)
}

func (e *EnforceSecurityImpl) ReadPartner(ctx context.Context, partnerId string) error {
	err := e.Permission(models.PARTNER_LIST)
	if err == nil {
		return nil
	}

	return errors.Join(
		e.Permission(models.PARTNER_READ),
		utils.EnforcePartnerAccess(e.Credentials, partnerId),
	)
}

func (e *EnforceSecurityImpl) UpdatePartner(ctx context.Context) error {
	return errors.Join(
		e.Permission(models.PARTNER_UPDATE),
	)
}
