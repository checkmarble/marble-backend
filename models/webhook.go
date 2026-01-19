package models

import (
	"net/url"
	"slices"
	"time"

	"github.com/google/uuid"
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
	// The webhooks feature is available in the license, or no convoy server has been set up
	Skipped WebhookEventDeliveryStatus = "skipped"
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
	WebhookEventType_CaseRuleSnoozeCreated,
	WebhookEventType_CaseDecisionReviewed,
}

type WebhookEventContent struct {
	Type WebhookEventType
	Data WebhookEventPayload
}

type WebhookEventPayload struct {
	Type      WebhookEventType
	Content   WebhookEventData
	Timestamp time.Time
}

type WebhookEventData struct {
	Decision *DecisionWithRuleExecutions
	Case     *Case
	Files    *[]CaseFile
	Comments *CaseEvent
}

type WebhookEvent struct {
	Id             string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	RetryCount     int
	DeliveryStatus WebhookEventDeliveryStatus
	OrganizationId uuid.UUID
	PartnerId      null.String
	EventContent   WebhookEventContent
}

type WebhookEventCreate struct {
	Id             string
	OrganizationId uuid.UUID
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
	OrganizationId *uuid.UUID
}

func (f WebhookEventFilters) MergeWithDefaults() WebhookEventFilters {
	defaultFilters := WebhookEventFilters{
		Limit: 100,
	}
	defaultFilters.DeliveryStatus = f.DeliveryStatus
	defaultFilters.OrganizationId = f.OrganizationId
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

func NewWebhookEventDecisionCreated(decision DecisionWithRuleExecutions) WebhookEventContent {
	return WebhookEventContent{
		Type: WebhookEventType_DecisionCreated,
		Data: WebhookEventPayload{
			Type:      WebhookEventType_DecisionCreated,
			Content:   WebhookEventData{Decision: &decision},
			Timestamp: time.Now(),
		},
	}
}

func newWebhookContent(eventType WebhookEventType, p WebhookEventData) WebhookEventContent {
	return WebhookEventContent{
		Type: eventType,
		Data: WebhookEventPayload{
			Type:      eventType,
			Content:   p,
			Timestamp: time.Now(),
		},
	}
}

func NewWebhookEventCaseUpdated(c Case) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseUpdated, WebhookEventData{Case: &c})
}

func NewWebhookEventCaseCreatedManually(c Case) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseCreatedManually, WebhookEventData{Case: &c})
}

func NewWebhookEventCaseCreatedWorkflow(c Case) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseCreatedWorkflow, WebhookEventData{Case: &c})
}

func NewWebhookEventCaseDecisionsUpdated(c Case) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseDecisionsUpdated, WebhookEventData{Case: &c})
}

func NewWebhookEventCaseTagsUpdated(c Case) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseTagsUpdated, WebhookEventData{Case: &c})
}

func NewWebhookEventCaseCommentCreated(c Case, comments CaseEvent) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseCommentCreated, WebhookEventData{Case: &c, Comments: &comments})
}

func NewWebhookEventCaseFileCreated(c Case, files []CaseFile) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseFileCreated, WebhookEventData{Case: &c, Files: &files})
}

func NewWebhookEventRuleSnoozeCreated(c Case, ruleSnooze RuleSnooze) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseRuleSnoozeCreated, WebhookEventData{Case: &c})
}

func NewWebhookEventDecisionReviewed(c Case, decision Decision) WebhookEventContent {
	return newWebhookContent(WebhookEventType_CaseDecisionReviewed, WebhookEventData{Case: &c, Decision: &DecisionWithRuleExecutions{Decision: decision}})
}

type Webhook struct {
	Id                string
	OrganizationId    uuid.UUID
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
