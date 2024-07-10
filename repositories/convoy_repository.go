package repositories

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	convoy "github.com/frain-dev/convoy-go/v2"
)

type ConvoyClientProvider interface {
	GetClient() (*convoy.Client, error)
}

type ConvoyRepository struct {
	convoyClientProvider ConvoyClientProvider
}

func getOwnerId(webhook models.Webhook) string {
	if webhook.PartnerId.Valid {
		return fmt.Sprintf("org:%s-partner:%s", webhook.OrganizationId, webhook.PartnerId.String)
	}
	return fmt.Sprintf("org:%s", webhook.OrganizationId)
}

func (repo ConvoyRepository) SendWebhookEvent(ctx context.Context, webhook models.Webhook) error {
	eventData, err := json.Marshal(webhook.EventData)
	if err != nil {
		return fmt.Errorf("can't encode webhook's event data: %v", err)
	}

	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return fmt.Errorf("can't get convoy client: %v", err)
	}

	err = convoyClient.Events.FanoutEvent(ctx, &convoy.CreateFanoutEventRequest{
		OwnerID:        getOwnerId(webhook),
		EventType:      webhook.EventType.String(),
		IdempotencyKey: webhook.Id,
		Data:           eventData,
	})
	if err != nil {
		return fmt.Errorf("can't create convoy event: %v", err)
	}
	return nil
}
