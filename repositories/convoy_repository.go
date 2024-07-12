package repositories

import (
	"context"
	"encoding/json"
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

func (repo ConvoyRepository) SendWebhookEvent(ctx context.Context, webhookEvent models.WebhookEvent) error {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return err
	}

	ownerId := getOwnerId(webhookEvent.OrganizationId, webhookEvent.PartnerId)
	eventType := string(webhookEvent.EventContent.Type)

	fanoutEvent, err := convoyClient.CreateEndpointFanoutEventWithResponse(ctx, projectId, convoy.ModelsFanoutEvent{
		OwnerId:        &ownerId,
		EventType:      &eventType,
		IdempotencyKey: &webhookEvent.Id,
		Data:           &webhookEvent.EventContent.Data,
	})
	if err != nil {
		return errors.Wrap(err, "can't create convoy event: request error")
	}
	if fanoutEvent.JSON201 == nil {
		err = parseResponseError(fanoutEvent.HTTPResponse.Status, fanoutEvent.Body)
		return errors.Wrap(err, "can't create convoy event")
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
	if endpoint.JSON201 != nil {
		err = parseResponseError(endpoint.HTTPResponse.Status, endpoint.Body)
		return errors.Wrap(err, "can't create convoy endpoint")
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
		err = parseResponseError(endpoint.HTTPResponse.Status, endpoint.Body)
		return errors.Wrap(err, "can't create convoy subscription")
	}

	return nil
}

func (repo ConvoyRepository) ListWebhooks(ctx context.Context, organizationId string, partnerId null.String) ([]models.Webhook, error) {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return nil, err
	}

	ownerId := getOwnerId(organizationId, partnerId)

	endpoints, err := convoyClient.GetEndpointsWithResponse(ctx, projectId, &convoy.GetEndpointsParams{
		OwnerId: &ownerId,
		PerPage: utils.Ptr(100),
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't get convoy endpoints: request error")
	}
	if endpoints.JSON200 == nil {
		err = parseResponseError(endpoints.HTTPResponse.Status, endpoints.Body)
		return nil, errors.Wrap(err, "can't get convoy endpoints")
	}

	endpointMap := make(map[string]convoy.ModelsEndpointResponse)
	endpointIds := make([]string, 0, len(*endpoints.JSON200.Data.Content))
	for _, convoyEndpoint := range *endpoints.JSON200.Data.Content {
		endpointIds = append(endpointIds, *convoyEndpoint.Uid)
		endpointMap[*convoyEndpoint.Uid] = convoyEndpoint
	}

	subscriptions, err := convoyClient.GetSubscriptionsWithResponse(ctx, projectId, &convoy.GetSubscriptionsParams{
		EndpointId: &endpointIds,
		PerPage:    utils.Ptr(100),
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't get convoy subscriptions: request error")
	}
	if subscriptions.JSON200 == nil {
		err = parseResponseError(subscriptions.HTTPResponse.Status, subscriptions.Body)
		return nil, errors.Wrap(err, "can't get convoy subscriptions")
	}

	webhooks := make([]models.Webhook, 0, len(*subscriptions.JSON200.Data.Content))
	for _, convoySubscription := range *subscriptions.JSON200.Data.Content {
		convoyEndpoint, ok := endpointMap[*convoySubscription.EndpointMetadata.Uid]
		if !ok {
			return nil, errors.New("can't find convoy endpoint")
		}

		webhooks = append(webhooks, adaptWebhook(convoyEndpoint, convoySubscription))
	}

	return webhooks, nil
}

func adaptSecret(convoySecret convoy.DatastoreSecret) models.Secret {
	secret := models.Secret{
		CreatedAt: *convoySecret.CreatedAt,
		Uid:       *convoySecret.Uid,
		UpdatedAt: *convoySecret.UpdatedAt,
		Value:     *convoySecret.Value,
	}
	if convoySecret.DeletedAt != nil {
		secret.DeletedAt = *convoySecret.DeletedAt
	}
	if convoySecret.ExpiresAt != nil {
		secret.ExpiresAt = *convoySecret.ExpiresAt
	}
	return secret
}

func adaptWebhook(
	convoyEndpoint convoy.ModelsEndpointResponse,
	convoySubscription convoy.ModelsSubscriptionResponse,
) models.Webhook {
	webhook := models.Webhook{
		SubscriptionId:    *convoySubscription.Uid,
		EndpointId:        *convoyEndpoint.Uid,
		EventTypes:        *convoySubscription.FilterConfig.EventTypes,
		Url:               *convoyEndpoint.Url,
		HttpTimeout:       convoyEndpoint.HttpTimeout,
		RateLimit:         convoyEndpoint.RateLimit,
		RateLimitDuration: convoyEndpoint.RateLimitDuration,
	}

	if convoyEndpoint.Secrets != nil {
		for _, convoySecret := range *convoyEndpoint.Secrets {
			webhook.Secrets = append(webhook.Secrets, adaptSecret(convoySecret))
		}
	}

	return webhook
}

func parseResponseError(status string, body []byte) error {
	var dest struct {
		Message *string `json:"message,omitempty"`
	}
	err := json.Unmarshal(body, &dest)
	if err != nil || dest.Message == nil {
		return errors.New(status)
	}
	return errors.Newf("%s: %s", status, *dest.Message)
}
