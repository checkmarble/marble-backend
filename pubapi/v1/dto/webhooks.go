package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type WebhookEventPayload struct {
	Type      string           `json:"type"`
	Content   WebhookEventData `json:"content"`
	Timestamp time.Time        `json:"timestamp"`
}

func AdaptWebhookEventData(m models.WebhookEventPayload) (json.RawMessage, error) {
	payload := WebhookEventPayload{
		Type: string(m.Type),
		Content: WebhookEventData{
			Decision: WebhookPayloadId(m.Content.Decision),
			Case:     WebhookPayloadId(m.Content.Case),
		},
		Timestamp: m.Timestamp,
	}

	out, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return out, nil
}

type WebhookEventData struct {
	Decision WebhookPayloadId `json:"decision,omitzero"`
	Case     WebhookPayloadId `json:"case,omitzero"`
}

type WebhookPayloadId struct {
	Id string `json:"id"`
}
