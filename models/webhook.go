package models

import (
	"fmt"
	"time"

	"github.com/guregu/null/v5"
)

type WebhookEventDeliveryStatus int

const (
	// In this state, the event delivery has been enqueued to the message broker, but a worker node is yet to pick it up for delivery.
	Scheduled WebhookEventDeliveryStatus = iota
	// The event has been successfully delivered to the target service.
	Success
	// The event delivery previously failed and the automatic retries have kicked in
	Retry
	// The event delivery has reached the maximum amount of automatic retries and failed to deliver the event or the endpoint failed to acknowledge delivery
	Failed
)

func (webhookEventDeliveryStatus WebhookEventDeliveryStatus) String() string {
	switch webhookEventDeliveryStatus {
	case Scheduled:
		return "scheduled"
	case Success:
		return "success"
	case Retry:
		return "retry"
	case Failed:
		return "failed"
	}
	panic(fmt.Errorf("unknown webhook event delivery status: %d", webhookEventDeliveryStatus))
}

func WebhookEventDeliveryStatusFrom(s string) WebhookEventDeliveryStatus {
	switch s {
	case "scheduled":
		return Scheduled
	case "success":
		return Success
	case "retry":
		return Retry
	case "failed":
		return Failed
	}
	panic(fmt.Errorf("unknown webhook event delivery status: %s", s))
}

type WebhookEventType int

const (
	WebhookEventType_CaseStatusUpdated WebhookEventType = iota
)

func (webhookEventType WebhookEventType) String() string {
	switch webhookEventType {
	case WebhookEventType_CaseStatusUpdated:
		return "case_status_updated"
	}
	panic(fmt.Errorf("unknown webhook event type: %d", webhookEventType))
}

func WebhookEventTypeFrom(s string) WebhookEventType {
	switch s {
	case "case_status_updated":
		return WebhookEventType_CaseStatusUpdated
	}
	panic(fmt.Errorf("unknown webhook event type: %s", s))
}

type WebhookEvent struct {
	Id               string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	SendAttemptCount int
	DeliveryStatus   WebhookEventDeliveryStatus
	OrganizationId   string
	PartnerId        null.String
	EventType        WebhookEventType
	EventData        map[string]any
}

type WebhookEventCreate struct {
	OrganizationId string
	PartnerId      null.String
	EventType      WebhookEventType
	EventData      map[string]any
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
