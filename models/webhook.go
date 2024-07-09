package models

import (
	"fmt"
	"time"

	"github.com/guregu/null/v5"
)

type WebhookDeliveryStatus int

const (
	// In this state, the event delivery has been enqueued to the message broker, but a worker node is yet to pick it up for delivery.
	Scheduled WebhookDeliveryStatus = iota
	// The event has been successfully delivered to the target service.
	Success
	// The event delivery previously failed and the automatic retries have kicked in
	Retry
	// The event delivery has reached the maximum amount of automatic retries and failed to deliver the event or the endpoint failed to acknowledge delivery
	Failed
)

func (webhookDeliveryStatus WebhookDeliveryStatus) String() string {
	switch webhookDeliveryStatus {
	case Scheduled:
		return "scheduled"
	case Success:
		return "success"
	case Retry:
		return "retry"
	case Failed:
		return "failed"
	}
	panic(fmt.Errorf("unknown webhook delivery status: %d", webhookDeliveryStatus))
}

func WebhookDeliveryStatusFrom(s string) WebhookDeliveryStatus {
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
	panic(fmt.Errorf("unknown webhook delivery status: %s", s))
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

type Webhook struct {
	Id               string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	SendAttemptCount int
	DeliveryStatus   WebhookDeliveryStatus
	OrganizationId   string
	PartnerId        null.String
	EventType        WebhookEventType
	EventData        map[string]any
}

type WebhookCreate struct {
	OrganizationId string
	PartnerId      null.String
	EventType      WebhookEventType
	EventData      map[string]any
}

type WebhookUpdate struct {
	Id               string
	UpdatedAt        time.Time
	DeliveryStatus   WebhookDeliveryStatus
	SendAttemptCount int
}

type WebhookFilters struct {
	DeliveryStatus []WebhookDeliveryStatus
}
