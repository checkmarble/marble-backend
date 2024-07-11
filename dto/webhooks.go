package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type WebhookCreateBody struct {
	EventType         string `json:"event_type"`
	Url               string `json:"url"`
	HttpTimeout       *int   `json:"http_timeout,omitempty"`
	RateLimit         *int   `json:"rate_limit,omitempty"`
	RateLimitDuration *int   `json:"rate_limit_duration,omitempty"`
}

func AdaptWebhookCreate(organizationId string, partnerId *string, input WebhookCreateBody) models.WebhookCreate {
	return models.WebhookCreate{
		OrganizationId:    organizationId,
		PartnerId:         null.StringFromPtr(partnerId),
		EventType:         models.WebhookEventType(input.EventType),
		Url:               input.Url,
		HttpTimeout:       input.HttpTimeout,
		RateLimit:         input.RateLimit,
		RateLimitDuration: input.RateLimitDuration,
	}
}
