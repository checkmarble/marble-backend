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
)

type WebhookEventType string

const (
	WebhookEventType_CaseUpdated           WebhookEventType = "case.updated"
	WebhookEventType_CaseCreatedManually   WebhookEventType = "case.created_manually"
	WebhookEventType_CaseCreatedWorkflow   WebhookEventType = "case.created_from_workflow"
	WebhookEventType_CaseDecisionsUpdated  WebhookEventType = "case.decisions_updated"
	WebhookEventType_CaseTagsUpdated       WebhookEventType = "case.tags_updated"
	WebhookEventType_CaseCommentCreated    WebhookEventType = "case.comment_created"
	WebhookEventType_CaseFileCreated       WebhookEventType = "case.file_created"
	WebhookEventType_CaseRuleSnoozeCreated WebhookEventType = "case.rule_snooze_created"
	WebhookEventType_CaseDecisionReviewed  WebhookEventType = "case.decision_reviewed"
	WebhookEventType_DecisionCreated       WebhookEventType = "decision.created"
)

var validWebhookEventTypes = []WebhookEventType{
	WebhookEventType_CaseUpdated,
	WebhookEventType_CaseCreatedManually,
	WebhookEventType_CaseCreatedWorkflow,
	WebhookEventType_CaseDecisionsUpdated,
	WebhookEventType_CaseTagsUpdated,
	WebhookEventType_CaseCommentCreated,
	WebhookEventType_CaseFileCreated,
	WebhookEventType_DecisionCreated,
}

type WebhookEventContent struct {
	Type WebhookEventType
	Data map[string]any
}

type WebhookEvent struct {
	Id             string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RetryCount     int
	DeliveryStatus WebhookEventDeliveryStatus
	OrganizationId string
	PartnerId      null.String
	EventContent   WebhookEventContent
}

type WebhookEventCreate struct {
	Id             string
	OrganizationId string
	PartnerId      null.String
	EventContent   WebhookEventContent
}

type WebhookEventUpdate struct {
	Id             string
	DeliveryStatus WebhookEventDeliveryStatus
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
	if input.HttpTimeout != nil && *input.HttpTimeout < 0 {
		return errors.Wrapf(BadParameterError, "invalid HttpTimeout: %d", *input.HttpTimeout)
	}
	if input.RateLimit != nil && *input.RateLimit < 0 {
		return errors.Wrapf(BadParameterError, "invalid RateLimit: %d", *input.RateLimit)
	}
	if input.RateLimitDuration != nil && *input.RateLimitDuration < 0 {
		return errors.Wrapf(BadParameterError, "invalid RateLimitDuration: %d", *input.RateLimitDuration)
	}

	return nil
}

func NewWebhookEventDecisionCreated(id string) WebhookEventContent {
	return WebhookEventContent{
		Type: WebhookEventType_DecisionCreated,
		Data: map[string]any{
			"type":      WebhookEventType_DecisionCreated,
			"content":   map[string]any{"decision": map[string]any{"id": id}},
			"timestamp": time.Now(),
		},
	}
}

func mapOfCaseWithId(id string) map[string]any {
	return map[string]any{"case": map[string]any{"id": id}}
}

func newWebhookContentCase(eventType WebhookEventType, id string) WebhookEventContent {
	return WebhookEventContent{
		Type: eventType,
		Data: map[string]any{
			"type":      eventType,
			"content":   mapOfCaseWithId(id),
			"timestamp": time.Now(),
		},
	}
}

func NewWebhookEventCaseUpdated(c Case) WebhookEventContent {
	return newWebhookContentCase(WebhookEventType_CaseUpdated, c.Id)
}

func NewWebhookEventCaseCreatedManually(c CaseMetadata) WebhookEventContent {
	return newWebhookContentCase(WebhookEventType_CaseCreatedManually, c.Id)
}

func NewWebhookEventCaseCreatedWorkflow(c CaseMetadata) WebhookEventContent {
	return newWebhookContentCase(WebhookEventType_CaseCreatedWorkflow, c.Id)
}

func NewWebhookEventCaseDecisionsUpdated(c CaseMetadata) WebhookEventContent {
	return newWebhookContentCase(WebhookEventType_CaseDecisionsUpdated, c.Id)
}

func NewWebhookEventCaseTagsUpdated(c Case) WebhookEventContent {
	return newWebhookContentCase(WebhookEventType_CaseTagsUpdated, c.Id)
}

func NewWebhookEventCaseCommentCreated(c Case) WebhookEventContent {
	return newWebhookContentCase(WebhookEventType_CaseCommentCreated, c.Id)
}

func NewWebhookEventCaseFileCreated(c Case) WebhookEventContent {
	return newWebhookContentCase(WebhookEventType_CaseFileCreated, c.Id)
}

func NewWebhookEventRuleSnoozeCreated(c Case) WebhookEventContent {
	return newWebhookContentCase(WebhookEventType_CaseRuleSnoozeCreated, c.Id)
}

func NewWebhookEventDecisionReviewed(c Case, decisionId string) WebhookEventContent {
	return WebhookEventContent{
		Type: WebhookEventType_CaseDecisionReviewed,
		Data: map[string]any{
			"type": WebhookEventType_CaseDecisionReviewed,
			"content": map[string]any{
				"case":     map[string]any{"id": c.Id},
				"decision": map[string]any{"id": decisionId},
			},
			"timestamp": time.Now(),
		},
	}
}

type Webhook struct {
	Id                string
	OrganizationId    string
	PartnerId         null.String
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

type WebhookUpdate struct {
	EventTypes        *[]string
	Url               *string
	HttpTimeout       *int
	RateLimit         *int
	RateLimitDuration *int
}

func (input WebhookUpdate) Validate() error {
	if input.EventTypes != nil {
		for _, eventType := range *input.EventTypes {
			if !slices.Contains(validWebhookEventTypes, WebhookEventType(eventType)) {
				return errors.Wrapf(BadParameterError, "invalid event type: %s", eventType)
			}
		}
	}
	if input.Url != nil {
		if _, err := url.ParseRequestURI(*input.Url); err != nil {
			return errors.Wrapf(BadParameterError, "invalid Url: %s", *input.Url)
		}
	}
	if input.HttpTimeout != nil && *input.HttpTimeout < 0 {
		return errors.Wrapf(BadParameterError, "invalid HttpTimeout: %d", *input.HttpTimeout)
	}
	if input.RateLimit != nil && *input.RateLimit < 0 {
		return errors.Wrapf(BadParameterError, "invalid RateLimit: %d", *input.RateLimit)
	}
	if input.RateLimitDuration != nil && *input.RateLimitDuration < 0 {
		return errors.Wrapf(BadParameterError, "invalid RateLimitDuration: %d", *input.RateLimitDuration)
	}
	return nil
}

// MergeWebhookWithUpdate merges a Webhook with a WebhookUpdate, returning a new Webhook with the updated fields.
// Secret is not updated by this function.
func MergeWebhookWithUpdate(w Webhook, update WebhookUpdate) Webhook {
	result := Webhook{
		Id:                w.Id,
		OrganizationId:    w.OrganizationId,
		PartnerId:         w.PartnerId,
		EventTypes:        w.EventTypes,
		Url:               w.Url,
		HttpTimeout:       w.HttpTimeout,
		RateLimit:         w.RateLimit,
		RateLimitDuration: w.RateLimitDuration,
	}
	if update.EventTypes != nil {
		result.EventTypes = *update.EventTypes
	}
	if update.Url != nil {
		result.Url = *update.Url
	}
	if update.HttpTimeout != nil {
		result.HttpTimeout = update.HttpTimeout
	}
	if update.RateLimit != nil {
		result.RateLimit = update.RateLimit
	}
	if update.RateLimitDuration != nil {
		result.RateLimitDuration = update.RateLimitDuration
	}
	return result
}
