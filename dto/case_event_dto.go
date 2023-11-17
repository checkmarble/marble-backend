package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APICaseEvent struct {
	Id             string    `json:"id"`
	CaseId         string    `json:"case_id"`
	User           User      `json:"user"`
	CreatedAt      time.Time `json:"created_at"`
	EventType      string    `json:"event_type"`
	AdditionalNote string    `json:"additional_note"`
	NewValue       string    `json:"new_value"`
}

func NewAPICaseEvent(caseEvent models.CaseEvent) APICaseEvent {
	return APICaseEvent{
		Id:             caseEvent.Id,
		CaseId:         caseEvent.CaseId,
		User:           AdaptUserDto(caseEvent.User),
		CreatedAt:      caseEvent.CreatedAt,
		EventType:      string(caseEvent.EventType),
		AdditionalNote: caseEvent.AdditionalNote,
		NewValue:       caseEvent.NewValue,
	}
}
