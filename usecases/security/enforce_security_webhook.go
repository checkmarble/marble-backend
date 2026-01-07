package security

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (e *EnforceSecurityImpl) SendWebhookEvent(ctx context.Context, organizationId string, partnerId null.String) error {
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.WEBHOOK_EVENT),
		utils.EnforceOrganizationAndPartnerAccess(e.Credentials, orgId, partnerId),
	)
}

func (e *EnforceSecurityImpl) CanCreateWebhook(ctx context.Context, organizationId string, partnerId null.String) error {
	orgId, _ := uuid.Parse(organizationId)
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAndPartnerAccess(e.Credentials, orgId, partnerId),
	)
}

func (e *EnforceSecurityImpl) CanReadWebhook(ctx context.Context, webhook models.Webhook) error {
	orgId, _ := uuid.Parse(webhook.OrganizationId)
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAndPartnerAccess(e.Credentials, orgId, webhook.PartnerId),
	)
}

func (e *EnforceSecurityImpl) CanModifyWebhook(ctx context.Context, webhook models.Webhook) error {
	orgId, _ := uuid.Parse(webhook.OrganizationId)
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAndPartnerAccess(e.Credentials, orgId, webhook.PartnerId),
	)
}
