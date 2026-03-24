package security

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func (e *EnforceSecurityImpl) SendWebhookEvent(ctx context.Context, organizationId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.WEBHOOK_EVENT),
		utils.EnforceOrganizationAccess(e.Credentials, organizationId),
	)
}

func (e *EnforceSecurityImpl) CanCreateWebhook(ctx context.Context, organizationId uuid.UUID) error {
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAccess(e.Credentials, organizationId),
	)
}

func (e *EnforceSecurityImpl) CanReadWebhook(ctx context.Context, webhook models.Webhook) error {
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAccess(e.Credentials, webhook.OrganizationId),
	)
}

func (e *EnforceSecurityImpl) CanModifyWebhook(ctx context.Context, webhook models.Webhook) error {
	return errors.Join(
		e.Permission(models.WEBHOOK),
		utils.EnforceOrganizationAccess(e.Credentials, webhook.OrganizationId),
	)
}
