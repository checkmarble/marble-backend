package security

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type EnforceSecuritySanctionCheck interface {
	EnforceSecurity

	ReadWhitelist(ctx context.Context) error
	WriteWhitelist(ctx context.Context) error
	PerformFreeformSearch(ctx context.Context) error
}

func (e *EnforceSecurityImpl) ReadWhitelist(ctx context.Context) error {
	return e.Permission(models.SANCTION_CHECK_WHITELIST_READ)
}

func (e *EnforceSecurityImpl) WriteWhitelist(ctx context.Context) error {
	return e.Permission(models.SANCTION_CHECK_WHITELIST_WRITE)
}

func (e *EnforceSecurityImpl) PerformFreeformSearch(ctx context.Context) error {
	return e.Permission(models.SANCTION_CHECK_FREEFORM_SEARCH)
}
