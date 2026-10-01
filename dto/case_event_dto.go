package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
)

type APICaseEvent struct {
	Id             string      `json:"id"`
	CaseId         string      `json:"case_id"`
	UserId         null.String `json:"user_id"`
	CreatedAt      time.Time   `json:"created_at"`
	EventType      string      `json:"event_type"`
	AdditionalNote string      `json:"additional_note"`
	PreviousValue  string      `json:"previous_value"`
	NewValue       string      `json:"new_value"`
	ResourceType   string      `json:"resource_type"`
	ResourceId     string      `json:"resource_id"`
	InboxId        *uuid.UUID  `json:"inbox_id,omitempty"`
}

func NewAPICaseEvent(caseEvent models.CaseEvent) APICaseEvent {
	return APICaseEvent{
		Id:             caseEvent.Id,
		CaseId:         caseEvent.CaseId,
		UserId:         caseEvent.UserId,
		CreatedAt:      caseEvent.CreatedAt,
		EventType:      string(caseEvent.EventType),
		AdditionalNote: caseEvent.AdditionalNote,
		NewValue:       caseEvent.NewValue,
		PreviousValue:  caseEvent.PreviousValue,
		ResourceType:   string(caseEvent.ResourceType),
		ResourceId:     caseEvent.ResourceId,
	}
}

func AdaptCaseEvents(caseEvents []models.CaseEvent, currentInboxId uuid.UUID) []APICaseEvent {
	inboxId := currentInboxId

	return pure_utils.Map(caseEvents, func(caseEvent models.CaseEvent) APICaseEvent {
		event := NewAPICaseEvent(caseEvent)
		if caseEvent.EventType != models.CaseEscalated && caseEvent.EventType != models.CaseInboxChanged {
			eventInboxId := inboxId
			event.InboxId = &eventInboxId
		}

		if caseEvent.EventType == models.CaseInboxChanged {
			if previousInboxId, err := uuid.Parse(caseEvent.PreviousValue); err == nil {
				inboxId = previousInboxId
			}
		}

		return event
	})
}
