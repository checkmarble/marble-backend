package security

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (e *EnforceSecurityImpl) SendWebhookEvent(ctx context.Context, organizationId string, partnerId null.String) error {
	err := errors.Join(
		e.Permission(models.WEBHOOK_EVENT),
		utils.EnforceOrganizationAccess(e.Credentials, organizationId),
	)
	if partnerId.Valid {
		err = errors.Join(err, utils.EnforcePartnerAccess(e.Credentials, partnerId.String))
	}
	return err
}
