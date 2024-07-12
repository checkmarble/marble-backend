package repositories

import (
	"context"
	"fmt"
	"strings"

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

// Ensure event type is included in the event data
func getEventContent(eventContent models.WebhookEventContent) (string, map[string]interface{}) {
	eventType := string(eventContent.GetType())
	eventData := eventContent.GetData()
	eventData["event_type"] = eventType
	return eventType, eventData
}

func (repo ConvoyRepository) SendWebhookEvent(ctx context.Context, webhookEvent models.WebhookEvent) error {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return err
	}

	ownerId := getOwnerId(webhookEvent.OrganizationId, webhookEvent.PartnerId)
	eventType, eventData := getEventContent(webhookEvent.EventContent)

	fanoutEvent, err := convoyClient.CreateEndpointFanoutEventWithResponse(ctx, projectId, convoy.ModelsFanoutEvent{
		OwnerId:        &ownerId,
		EventType:      &eventType,
		IdempotencyKey: &webhookEvent.Id,
		Data:           &eventData,
	})
	if err != nil {
		return errors.Wrap(err, "can't create convoy event: request error")
	}
	if fanoutEvent.JSON201 == nil {
		return errors.New("can't create convoy event: response error")
	}

	return nil
}

func (repo ConvoyRepository) RegisterWebhook(ctx context.Context, input models.WebhookRegister) error {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return err
	}

	ownerId := getOwnerId(input.OrganizationId, input.PartnerId)

	eventLabel := "all-events"
	if len(input.EventTypes) > 0 {
		eventLabel = strings.Join(input.EventTypes, ",")
	}
	name := fmt.Sprintf("%s|%s", ownerId, eventLabel)

	endpoint, err := convoyClient.CreateEndpointWithResponse(ctx, projectId, convoy.ModelsCreateEndpoint{
		Name:              &name,
		OwnerId:           &ownerId,
		Url:               &input.Url,
		Secret:            &input.Secret,
		HttpTimeout:       input.HttpTimeout,
		RateLimit:         input.RateLimit,
		RateLimitDuration: input.RateLimitDuration,
	})
	if err != nil {
		return errors.Wrap(err, "can't create convoy endpoint: request error")
	}
	if endpoint.JSON201 == nil {
		return errors.New("can't create convoy endpoint: response error")
	}

	subscription, err := convoyClient.CreateSubscriptionWithResponse(ctx, projectId, convoy.ModelsCreateSubscription{
		Name:       &name,
		EndpointId: endpoint.JSON201.Data.Uid,
		FilterConfig: &convoy.ModelsFilterConfiguration{
			EventTypes: &input.EventTypes,
		},
		RetryConfig: &convoy.ModelsRetryConfiguration{
			Type:       utils.Ptr(convoy.ExponentialStrategyProvider),
			RetryCount: utils.Ptr(3),
			Duration:   utils.Ptr("3s"),
		},
	})
	if err != nil {
		return errors.Wrap(err, "can't create convoy subscription: request error")
	}
	if subscription.JSON201 == nil {
		return errors.New("can't create convoy subscription: response error")
	}

	return nil
}
