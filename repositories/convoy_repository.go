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
