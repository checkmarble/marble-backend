package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/guregu/null/v5"
)

type ConvoyClientProvider interface {
	GetClient() (convoy.Client, error)
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
	eventData, err := json.Marshal(webhookEvent.EventData)
	if err != nil {
		return errors.Wrap(err, "can't encode webhook event data")
	}

	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return err
	}

	err = convoyClient.Events.FanoutEvent(ctx, &convoy.CreateFanoutEventRequest{
		OwnerID:        getOwnerId(webhookEvent.OrganizationId, webhookEvent.PartnerId),
		EventType:      webhookEvent.EventType.String(),
		IdempotencyKey: webhookEvent.Id,
		Data:           eventData,
	})
	if err != nil {
		return errors.Wrap(err, "can't create convoy event")
	}
	return nil
}
