package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type WebhookRegisterBody struct {
	EventTypes        []string `json:"event_types,omitempty"`
	Url               string   `json:"url"`
	HttpTimeout       *int     `json:"http_timeout,omitempty"`
	RateLimit         *int     `json:"rate_limit,omitempty"`
	RateLimitDuration *int     `json:"rate_limit_duration,omitempty"`
}

type Webhook struct {
	Id                string   `json:"id"`
	EventTypes        []string `json:"event_types,omitempty"`
	Url               string   `json:"url"`
	HttpTimeout       *int     `json:"http_timeout,omitempty"`
	RateLimit         *int     `json:"rate_limit,omitempty"`
	RateLimitDuration *int     `json:"rate_limit_duration,omitempty"`
}

type WebhookWithSecret struct {
	Webhook
	Secrets []Secret `json:"secrets,omitempty"`
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
		Id:                webhook.Id,
		EventTypes:        webhook.EventTypes,
		Url:               webhook.Url,
		HttpTimeout:       webhook.HttpTimeout,
		RateLimit:         webhook.RateLimit,
		RateLimitDuration: webhook.RateLimitDuration,
	}
}

func AdaptWebhookWithSecret(webhook models.Webhook) WebhookWithSecret {
	return WebhookWithSecret{
		Webhook: AdaptWebhook(webhook),
		Secrets: pure_utils.Map(webhook.Secrets, AdaptSecret),
	}
}

type WebhookUpdateBody struct {
	EventTypes        *[]string `json:"event_types,omitempty"`
	Url               *string   `json:"url,omitempty"`
	HttpTimeout       *int      `json:"http_timeout,omitempty"`
	RateLimit         *int      `json:"rate_limit,omitempty"`
	RateLimitDuration *int      `json:"rate_limit_duration,omitempty"`
}
