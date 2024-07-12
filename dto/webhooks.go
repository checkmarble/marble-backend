package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
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

type Webhook struct {
	EndpointId        string   `json:"endpoint_id"`
	SubscriptionId    string   `json:"subscription_id"`
	EventTypes        []string `json:"event_types"`
	Secrets           []Secret `json:"secrets"`
	Url               string   `json:"url"`
	HttpTimeout       *int     `json:"http_timeout,omitempty"`
	RateLimit         *int     `json:"rate_limit,omitempty"`
	RateLimitDuration *int     `json:"rate_limit_duration,omitempty"`
}

type Secret struct {
	CreatedAt string `json:"created_at,omitempty"`
	DeletedAt string `json:"deleted_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Uid       string `json:"id,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Value     string `json:"value,omitempty"`
}

func AdaptSecret(secret models.Secret) Secret {
	return Secret{
		CreatedAt: secret.CreatedAt,
		DeletedAt: secret.DeletedAt,
		ExpiresAt: secret.ExpiresAt,
		Uid:       secret.Uid,
		UpdatedAt: secret.UpdatedAt,
		Value:     secret.Value,
	}
}

func AdaptWebhook(webhook models.Webhook) Webhook {
	return Webhook{
		EndpointId:        webhook.EndpointId,
		SubscriptionId:    webhook.SubscriptionId,
		EventTypes:        webhook.EventTypes,
		Secrets:           pure_utils.Map(webhook.Secrets, AdaptSecret),
		Url:               webhook.Url,
		HttpTimeout:       webhook.HttpTimeout,
		RateLimit:         webhook.RateLimit,
		RateLimitDuration: webhook.RateLimitDuration,
	}
}
