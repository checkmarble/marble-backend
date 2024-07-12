package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type WebhookRegisterBody struct {
	EventTypes        []string `json:"event_types"`
	Url               string   `json:"url"`
	HttpTimeout       *int     `json:"http_timeout,omitempty"`
	RateLimit         *int     `json:"rate_limit,omitempty"`
	RateLimitDuration *int     `json:"rate_limit_duration,omitempty"`
}

func AdaptWebhookRegister(organizationId string, partnerId *string, input WebhookRegisterBody) models.WebhookRegister {
	return models.WebhookRegister{
		OrganizationId:    organizationId,
		PartnerId:         null.StringFromPtr(partnerId),
		Url:               input.Url,
		EventTypes:        input.EventTypes,
		HttpTimeout:       input.HttpTimeout,
		RateLimit:         input.RateLimit,
		RateLimitDuration: input.RateLimitDuration,
	}
}
