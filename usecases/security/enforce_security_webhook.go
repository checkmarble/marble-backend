package security

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (e *EnforceSecurityImpl) CreateWebhook(ctx context.Context, organizationId string, partnerId null.String) error {
	err := errors.Join(
		e.Permission(models.WEBHOOK_CREATE),
		utils.EnforceOrganizationAccess(e.Credentials, organizationId),
	)
	if partnerId.Valid {
		err = errors.Join(err, utils.EnforcePartnerAccess(e.Credentials, partnerId.String))
	}
	return err
}

func (e *EnforceSecurityImpl) SendWebhook(
	ctx context.Context,
	webhook models.Webhook,
) error {
	err := errors.Join(
		e.Permission(models.WEBHOOK_SEND),
		utils.EnforceOrganizationAccess(e.Credentials, webhook.OrganizationId),
	)
	if webhook.PartnerId.Valid {
		err = errors.Join(err, utils.EnforcePartnerAccess(e.Credentials, webhook.PartnerId.String))
	}
	return err
}
