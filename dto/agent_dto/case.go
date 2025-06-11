package agent_dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/guregu/null/v5"
)

type CaseEvent struct {
	UserName       null.String `json:"user_name"`
	CreatedAt      time.Time   `json:"created_at"`
	EventType      string      `json:"event_type"`
	AdditionalNote string      `json:"additional_note"`
	NewValue       string      `json:"new_value"`
	ResourceType   string      `json:"resource_type"`
}

func AdaptCaseEventDto(caseEvent models.CaseEvent, users []models.User) CaseEvent {
	var userName null.String
	for _, user := range users {
		if user.UserId == models.UserId(caseEvent.UserId.String) {
			userName = null.StringFrom(user.FirstName + " " + user.LastName)
			break
		}
	}
	return CaseEvent{
		UserName:       userName,
		CreatedAt:      caseEvent.CreatedAt,
		EventType:      string(caseEvent.EventType),
		AdditionalNote: caseEvent.AdditionalNote,
		NewValue:       caseEvent.NewValue,
		ResourceType:   string(caseEvent.ResourceType),
	}
}
