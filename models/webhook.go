package models

import (
	"net/url"
	"slices"
	"time"

	"github.com/guregu/null/v5"
	"github.com/pkg/errors"
)

type WebhookEventDeliveryStatus string

const (
	// In this state, the event delivery has been enqueued to the message broker, but a worker node is yet to pick it up for delivery.
	Scheduled WebhookEventDeliveryStatus = "scheduled"
	// The event has been successfully delivered to the target service.
	Success WebhookEventDeliveryStatus = "success"
	// The event delivery previously failed and the automatic retries have kicked in
	Retry WebhookEventDeliveryStatus = "retry"
	// The event delivery has reached the maximum amount of automatic retries and failed to deliver the event or the endpoint failed to acknowledge delivery
	Failed WebhookEventDeliveryStatus = "failed"
)

var validWebhookEventDeliveryStatus = []WebhookEventDeliveryStatus{
	Scheduled,
	Success,
	Retry,
	Failed,
}

type WebhookEventType string

const (
	WebhookEventType_CaseStatusUpdated WebhookEventType = "case_status_updated"
)

var validWebhookEventTypes = []WebhookEventType{
	WebhookEventType_CaseStatusUpdated,
}

type WebhookEventContent struct {
	Type WebhookEventType
	Data map[string]any
}

type WebhookEvent struct {
	Id               string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	SendAttemptCount int
	DeliveryStatus   WebhookEventDeliveryStatus
	OrganizationId   string
	PartnerId        null.String
	EventContent     WebhookEventContent
}

type WebhookEventCreate struct {
	OrganizationId string
	PartnerId      null.String
	EventContent   WebhookEventContent
}

type WebhookEventUpdate struct {
	Id               string
	DeliveryStatus   WebhookEventDeliveryStatus
	SendAttemptCount int
}

type WebhookEventFilters struct {
	DeliveryStatus []WebhookEventDeliveryStatus
	Limit          uint64
}

func (f WebhookEventFilters) MergeWithDefaults() WebhookEventFilters {
	defaultFilters := WebhookEventFilters{
		Limit: 100,
	}
	defaultFilters.DeliveryStatus = f.DeliveryStatus
	if f.Limit > 0 {
		defaultFilters.Limit = f.Limit
	}
	return defaultFilters
}

type WebhookRegister struct {
	OrganizationId    string
	PartnerId         null.String
	EventTypes        []string
	Secret            string
	Url               string
	HttpTimeout       *int
	RateLimit         *int
	RateLimitDuration *int
}

func (input WebhookRegister) Validate() error {
	for _, eventType := range input.EventTypes {
		if !slices.Contains(validWebhookEventTypes, WebhookEventType(eventType)) {
			return errors.Wrapf(BadParameterError, "invalid event type: %s", eventType)
		}
	}
	if _, err := url.ParseRequestURI(input.Url); err != nil {
		return errors.Wrapf(BadParameterError, "invalid Url: %s", input.Url)
	}

	return nil
}

func NewWebhookEventCaseStatusUpdated(caseStatus CaseStatus) WebhookEventContent {
	return WebhookEventContent{
		Type: WebhookEventType_CaseStatusUpdated,
		Data: map[string]any{
			"event_type":  WebhookEventType_CaseStatusUpdated,
			"case_status": caseStatus,
		},
	}
}

type Webhook struct {
	EndpointId        string
	SubscriptionId    string
	EventTypes        []string
	Secrets           []Secret
	Url               string
	HttpTimeout       *int
	RateLimit         *int
	RateLimitDuration *int
}

type Secret struct {
	CreatedAt string
	DeletedAt string
	ExpiresAt string
	Uid       string
	UpdatedAt string
	Value     string
}
