package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type APICaseEvent struct {
	Id             string      `json:"id"`
	CaseId         string      `json:"case_id"`
	UserId         null.String `json:"user_id"`
	CreatedAt      time.Time   `json:"created_at"`
	EventType      string      `json:"event_type"`
	AdditionalNote string      `json:"additional_note"`
	NewValue       string      `json:"new_value"`
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
	}
}
