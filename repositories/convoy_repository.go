package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/api-clients/convoy"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/guregu/null/v5"
)

type ConvoyClientProvider interface {
	GetClient() (convoy.ClientWithResponses, error)
	GetProjectID() string
}

type ConvoyRepository struct {
	convoyClientProvider ConvoyClientProvider
}

func getOwnerId(organizationId string, partnerId null.String) string {
	if partnerId.Valid {
		return fmt.Sprintf("org:%s-partner:%s", organizationId, partnerId.String)
	}
	return fmt.Sprintf("org:%s", organizationId)
}

func (repo ConvoyRepository) SendWebhookEvent(ctx context.Context, webhookEvent models.WebhookEvent) error {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return err
	}

	body := convoy.CreateEndpointFanoutEventJSONRequestBody{
		OwnerId:        utils.Ptr(getOwnerId(webhookEvent.OrganizationId, webhookEvent.PartnerId)),
		EventType:      utils.Ptr(webhookEvent.EventType.String()),
		IdempotencyKey: utils.Ptr(webhookEvent.Id),
		Data:           utils.Ptr(webhookEvent.EventData),
	}

	_, err = convoyClient.CreateEndpointFanoutEventWithResponse(ctx, projectId, body)
	if err != nil {
		return errors.Wrap(err, "can't create convoy event")
	}
	return nil
}
