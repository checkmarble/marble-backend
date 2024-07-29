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

type noOpConvoyClientProvider struct{}

func (noOpConvoyClientProvider) GetClient() (convoy.ClientWithResponses, error) {
	return convoy.ClientWithResponses{}, errors.New("convoy client provider is not set")
}

func (noOpConvoyClientProvider) GetProjectID() string {
	return ""
}

func NewConvoyRepository(convoyClientProvider ConvoyClientProvider) ConvoyRepository {
	if convoyClientProvider == nil {
		convoyClientProvider = noOpConvoyClientProvider{}
	}

	return ConvoyRepository{convoyClientProvider: convoyClientProvider}
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

func parseOwnerId(ownerId string) (string, null.String) {
	parts := strings.Split(ownerId, "-partner:")
	if len(parts) == 2 {
		return parts[0][4:], null.StringFrom(parts[1])
	}
	return ownerId[4:], null.String{}
}

func getName(ownerId string, eventTypes []string) string {
	eventLabel := "all-events"
	if len(eventTypes) > 0 {
		eventLabel = strings.Join(eventTypes, ",")
	}
	return fmt.Sprintf("%s|%s", ownerId, eventLabel)
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

func (repo ConvoyRepository) RegisterWebhook(
	ctx context.Context,
	organizationId string,
	partnerId null.String,
	input models.WebhookRegister,
) (models.Webhook, error) {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return models.Webhook{}, err
	}

	ownerId := getOwnerId(organizationId, partnerId)
	name := getName(ownerId, input.EventTypes)

	endpointRes, err := convoyClient.CreateEndpointWithResponse(ctx, projectId, convoy.ModelsCreateEndpoint{
		Name:              &name,
		OwnerId:           &ownerId,
		Url:               &input.Url,
		Secret:            &input.Secret,
		HttpTimeout:       input.HttpTimeout,
		RateLimit:         input.RateLimit,
		RateLimitDuration: input.RateLimitDuration,
	})
	if err != nil {
		return models.Webhook{}, errors.Wrap(err, "can't create convoy endpoint: request error")
	}
	if endpointRes.JSON201 == nil || endpointRes.JSON201.Data == nil {
		err = parseResponseError(endpointRes.HTTPResponse.Status, endpointRes.Body)
		return models.Webhook{}, errors.Wrap(err, "can't create convoy endpoint")
	}
	endpoint := *endpointRes.JSON201.Data

	filterConfig := adaptModelsFilterConfiguration(input.EventTypes)
	subscriptionRes, err := convoyClient.CreateSubscriptionWithResponse(ctx, projectId, convoy.ModelsCreateSubscription{
		Name:         &name,
		EndpointId:   endpoint.Uid,
		FilterConfig: &filterConfig,
		RetryConfig: &convoy.ModelsRetryConfiguration{
			Type:       utils.Ptr(convoy.ExponentialStrategyProvider),
			RetryCount: utils.Ptr(3),
			Duration:   utils.Ptr("3s"),
		},
	})
	if err != nil {
		return models.Webhook{}, errors.Wrap(err, "can't create convoy subscription: request error")
	}
	if subscriptionRes.JSON201 == nil || subscriptionRes.JSON201.Data == nil {
		err = parseResponseError(endpointRes.HTTPResponse.Status, endpointRes.Body)
		return models.Webhook{}, errors.Wrap(err, "can't create convoy subscription")
	}
	subscription := *subscriptionRes.JSON201.Data

	return adaptWebhook(endpoint, subscription), nil
}

var perPage = 1000

func checkPerPageLimit(ctx context.Context, count int) {
	if count >= perPage {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"convoy per page limit reached", "limit", perPage)
	}
}

func (repo ConvoyRepository) ListWebhooks(ctx context.Context, organizationId string, partnerId null.String) ([]models.Webhook, error) {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return nil, err
	}

	ownerId := getOwnerId(organizationId, partnerId)

	endpointRes, err := convoyClient.GetEndpointsWithResponse(ctx, projectId, &convoy.GetEndpointsParams{
		OwnerId: &ownerId,
		PerPage: &perPage,
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't get convoy endpoints: request error")
	}
	if endpointRes.JSON200 == nil {
		err = parseResponseError(endpointRes.HTTPResponse.Status, endpointRes.Body)
		return nil, errors.Wrap(err, "can't get convoy endpoints")
	}
	var endpoints []convoy.ModelsEndpointResponse
	if endpointRes.JSON200.Data.Content != nil {
		endpoints = *endpointRes.JSON200.Data.Content
	}
	checkPerPageLimit(ctx, len(endpoints))

	if len(endpoints) == 0 {
		return make([]models.Webhook, 0), nil
	}

	endpointMap := make(map[string]convoy.ModelsEndpointResponse)
	endpointIds := make([]string, 0, len(endpoints))
	for _, convoyEndpoint := range endpoints {
		endpointIds = append(endpointIds, *convoyEndpoint.Uid)
		endpointMap[*convoyEndpoint.Uid] = convoyEndpoint
	}

	subscriptionRes, err := convoyClient.GetSubscriptionsWithResponse(ctx, projectId, &convoy.GetSubscriptionsParams{
		EndpointId: &endpointIds,
		PerPage:    &perPage,
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't get convoy subscriptions: request error")
	}
	if subscriptionRes.JSON200 == nil {
		err = parseResponseError(subscriptionRes.HTTPResponse.Status, subscriptionRes.Body)
		return nil, errors.Wrap(err, "can't get convoy subscriptions")
	}
	var subscriptions []convoy.ModelsSubscriptionResponse
	if subscriptionRes.JSON200.Data.Content != nil {
		subscriptions = *subscriptionRes.JSON200.Data.Content
	}
	checkPerPageLimit(ctx, len(subscriptions))

	webhooks := make([]models.Webhook, 0, len(subscriptions))
	for _, convoySubscription := range subscriptions {
		convoyEndpoint, ok := endpointMap[*convoySubscription.EndpointMetadata.Uid]
		if !ok {
			return nil, errors.New("can't find convoy endpoint")
		}

		webhooks = append(webhooks, adaptWebhook(convoyEndpoint, convoySubscription))
	}

	return webhooks, nil
}

func adaptEventTypes(convoyFilterConfig convoy.DatastoreFilterConfiguration) []string {
	var eventTypes []string
	if convoyFilterConfig.EventTypes == nil || len(*convoyFilterConfig.EventTypes) == 0 {
		return eventTypes
	}
	if len(*convoyFilterConfig.EventTypes) == 1 && (*convoyFilterConfig.EventTypes)[0] == "*" {
		return eventTypes
	}
	eventTypes = append(eventTypes, *convoyFilterConfig.EventTypes...)

	return eventTypes
}

func adaptModelsFilterConfiguration(eventTypes []string) convoy.ModelsFilterConfiguration {
	var convoyEventTypes []string
	if len(eventTypes) > 0 {
		convoyEventTypes = eventTypes
	} else {
		convoyEventTypes = []string{"*"}
	}
	return convoy.ModelsFilterConfiguration{
		EventTypes: &convoyEventTypes,
	}
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
	organizationId, partnerId := parseOwnerId(*convoyEndpoint.OwnerId)

	webhook := models.Webhook{
		Id:                *convoyEndpoint.Uid,
		OrganizationId:    organizationId,
		PartnerId:         partnerId,
		Url:               *convoyEndpoint.Url,
		HttpTimeout:       convoyEndpoint.HttpTimeout,
		RateLimit:         convoyEndpoint.RateLimit,
		RateLimitDuration: convoyEndpoint.RateLimitDuration,
	}

	if convoySubscription.FilterConfig != nil {
		webhook.EventTypes = adaptEventTypes(*convoySubscription.FilterConfig)
	}

	if convoyEndpoint.Secrets != nil {
		for _, convoySecret := range *convoyEndpoint.Secrets {
			webhook.Secrets = append(webhook.Secrets, adaptSecret(convoySecret))
		}
	}

	return webhook
}

func getEndpoint(
	ctx context.Context,
	convoyClient convoy.ClientWithResponses,
	projectId string,
	endpointId string,
) (convoy.ModelsEndpointResponse, error) {
	endpointRes, err := convoyClient.GetEndpointWithResponse(ctx, projectId, endpointId)
	if err != nil {
		return convoy.ModelsEndpointResponse{},
			errors.Wrap(err, "can't get convoy endpoint: request error")
	}
	if endpointRes.JSON200 == nil {
		err = parseResponseError(endpointRes.HTTPResponse.Status, endpointRes.Body)
		return convoy.ModelsEndpointResponse{},
			errors.Wrap(err, "can't get convoy endpoint")
	}

	var endpoint convoy.ModelsEndpointResponse
	if endpointRes.JSON200.Data != nil {
		endpoint = *endpointRes.JSON200.Data
	}

	return endpoint, nil
}

func getSubscription(
	ctx context.Context,
	convoyClient convoy.ClientWithResponses,
	projectId string,
	endpointId string,
) (convoy.ModelsSubscriptionResponse, error) {
	subscriptionRes, err := convoyClient.GetSubscriptionsWithResponse(ctx, projectId, &convoy.GetSubscriptionsParams{
		EndpointId: &[]string{endpointId},
		PerPage:    &perPage,
	})
	if err != nil {
		return convoy.ModelsSubscriptionResponse{},
			errors.Wrap(err, "can't get convoy subscription: request error")
	}
	if subscriptionRes.JSON200 == nil {
		err = parseResponseError(subscriptionRes.HTTPResponse.Status, subscriptionRes.Body)
		return convoy.ModelsSubscriptionResponse{},
			errors.Wrap(err, "can't get convoy subscriptions")
	}

	var subscription convoy.ModelsSubscriptionResponse
	if subscriptionRes.JSON200.Data != nil &&
		subscriptionRes.JSON200.Data.Content != nil &&
		len(*subscriptionRes.JSON200.Data.Content) > 0 {
		subscription = (*subscriptionRes.JSON200.Data.Content)[0]
	} else {
		return convoy.ModelsSubscriptionResponse{},
			errors.New("can't find convoy subscription")
	}

	return subscription, nil
}

func (repo ConvoyRepository) GetWebhook(ctx context.Context, webhookId string) (models.Webhook, error) {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return models.Webhook{}, err
	}

	endpoint, err := getEndpoint(ctx, convoyClient, projectId, webhookId)
	if err != nil {
		return models.Webhook{}, err
	}

	subscription, err := getSubscription(ctx, convoyClient, projectId, webhookId)
	if err != nil {
		return models.Webhook{}, err
	}

	return adaptWebhook(endpoint, subscription), nil
}

func (repo ConvoyRepository) DeleteWebhook(ctx context.Context, webhookId string) error {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return err
	}

	// Delete linked subscription on cascade
	deleteRes, err := convoyClient.DeleteEndpointWithResponse(ctx, projectId, webhookId)
	if err != nil {
		return errors.Wrap(err, "can't delete convoy endpoint: request error")
	}
	if deleteRes.JSON200 == nil {
		err = parseResponseError(deleteRes.HTTPResponse.Status, deleteRes.Body)
		return errors.Wrap(err, "can't delete convoy endpoint")
	}

	return nil
}

func (repo ConvoyRepository) UpdateWebhook(
	ctx context.Context,
	input models.Webhook,
) (models.Webhook, error) {
	projectId := repo.convoyClientProvider.GetProjectID()
	convoyClient, err := repo.convoyClientProvider.GetClient()
	if err != nil {
		return models.Webhook{}, err
	}

	ownerId := getOwnerId(input.OrganizationId, input.PartnerId)
	name := getName(ownerId, input.EventTypes)

	subscription, err := getSubscription(ctx, convoyClient, projectId, input.Id)
	if err != nil {
		return models.Webhook{}, err
	}

	endpointRes, err := convoyClient.UpdateEndpointWithResponse(ctx, projectId, input.Id, convoy.ModelsUpdateEndpoint{
		Name:              &name,
		OwnerId:           &ownerId,
		Url:               &input.Url,
		HttpTimeout:       input.HttpTimeout,
		RateLimit:         input.RateLimit,
		RateLimitDuration: input.RateLimitDuration,
	})
	if err != nil {
		return models.Webhook{}, errors.Wrap(err, "can't update convoy endpoint: request error")
	}
	if endpointRes.JSON202 == nil || endpointRes.JSON202.Data == nil {
		err = parseResponseError(endpointRes.HTTPResponse.Status, endpointRes.Body)
		return models.Webhook{}, errors.Wrap(err, "can't update convoy endpoint")
	}
	endpoint := *endpointRes.JSON202.Data

	filterConfig := adaptModelsFilterConfiguration(input.EventTypes)
	subscriptionRes, err := convoyClient.UpdateSubscriptionWithResponse(ctx,
		projectId,
		*subscription.Uid,
		convoy.ModelsUpdateSubscription{
			FilterConfig: &filterConfig,
		})
	if err != nil {
		return models.Webhook{}, errors.Wrap(err, "can't update convoy subscription: request error")
	}
	if subscriptionRes.JSON202 == nil || subscriptionRes.JSON202.Data == nil {
		err = parseResponseError(subscriptionRes.HTTPResponse.Status, subscriptionRes.Body)
		return models.Webhook{}, errors.Wrap(err, "can't update convoy subscription")
	}
	subscription = *subscriptionRes.JSON202.Data

	return adaptWebhook(endpoint, subscription), nil
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
