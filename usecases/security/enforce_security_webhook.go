package security

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (e *EnforceSecurityImpl) SendWebhookEvent(ctx context.Context, organizationId string, partnerId null.String) error {
	return errors.Join(
		e.Permission(models.WEBHOOK_EVENT),
		utils.EnforceOrganizationAndPartnerAccess(e.Credentials, organizationId, partnerId),
	)
}

func (e *EnforceSecurityImpl) CanCreateWebhook(ctx context.Context, organizationId string, partnerId null.String) error {
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAndPartnerAccess(e.Credentials, organizationId, partnerId),
	)
}

func (e *EnforceSecurityImpl) CanReadWebhook(ctx context.Context, webhook models.Webhook) error {
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAndPartnerAccess(e.Credentials, webhook.OrganizationId, webhook.PartnerId),
	)
}

func (e *EnforceSecurityImpl) CanModifyWebhook(ctx context.Context, webhook models.Webhook) error {
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAndPartnerAccess(e.Credentials, webhook.OrganizationId, webhook.PartnerId),
	)
}
