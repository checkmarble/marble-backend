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

	ownerId := getOwnerId(webhookEvent.OrganizationId, webhookEvent.PartnerId)
	eventType := string(webhookEvent.EventType)

	_, err = convoyClient.CreateEndpointFanoutEventWithResponse(ctx, projectId, convoy.ModelsFanoutEvent{
		OwnerId:        &ownerId,
		EventType:      &eventType,
		IdempotencyKey: &webhookEvent.Id,
		Data:           &webhookEvent.EventData,
	})
	if err != nil {
		return errors.Wrap(err, "can't create convoy event")
	}
	return nil
}

func (repo ConvoyRepository) CreateWebhook(ctx context.Context, input models.WebhookCreate) error {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return err
	}

	ownerId := getOwnerId(input.OrganizationId, input.PartnerId)
	eventType := string(input.EventType)

	endpoint, err := convoyClient.CreateEndpointWithResponse(ctx, projectId, convoy.ModelsCreateEndpoint{
		Name:              &eventType,
		OwnerId:           &ownerId,
		Url:               &input.Url,
		Secret:            &input.Secret,
		HttpTimeout:       input.HttpTimeout,
		RateLimit:         input.RateLimit,
		RateLimitDuration: input.RateLimitDuration,
	})
	if err != nil {
		return errors.Wrap(err, "can't create convoy endpoint")
	}

	_, err = convoyClient.CreateSubscriptionWithResponse(ctx, projectId, convoy.ModelsCreateSubscription{
		Name:       utils.Ptr(fmt.Sprintf("%s-%s", ownerId, input.EventType)),
		EndpointId: endpoint.JSON201.Data.Uid,
		FilterConfig: &convoy.ModelsFilterConfiguration{
			EventTypes: &[]string{eventType},
		},
		RetryConfig: &convoy.ModelsRetryConfiguration{
			Type:       utils.Ptr(convoy.ExponentialStrategyProvider),
			RetryCount: utils.Ptr(3),
			Duration:   utils.Ptr("3s"),
		},
	})
	if err != nil {
		return errors.Wrap(err, "can't create convoy subscription")
	}

	return nil
}
