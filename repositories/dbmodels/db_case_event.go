package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBCaseEvent struct {
	Id             string    `db:"id"`
	CaseId         string    `db:"case_id"`
	UserId         string    `db:"user_id"`
	CreatedAt      time.Time `db:"created_at"`
	EventType      string    `db:"event_type"`
	AdditionalNote *string   `db:"additional_note"`
	ResourceId     *string   `db:"resource_id"`
	ResourceType   *string   `db:"resource_type"`
	NewValue       *string   `db:"new_value"`
	PreviousValue  *string   `db:"previous_value"`
}

const TABLE_CASE_EVENTS = "case_events"

var SelectCaseEventColumn = utils.ColumnList[DBCaseEvent]()

func AdaptCaseEvent(caseEvent DBCaseEvent, user models.User) models.CaseEvent {
	var additionalNote, resourceId, resourceType, newValue, previousValue string
	if caseEvent.AdditionalNote != nil {
		additionalNote = *caseEvent.AdditionalNote
	}
	if caseEvent.ResourceId != nil {
		resourceId = *caseEvent.ResourceId
	}
	if caseEvent.ResourceType != nil {
		resourceType = *caseEvent.ResourceType
	}
	if caseEvent.NewValue != nil {
		newValue = *caseEvent.NewValue
	}
	if caseEvent.PreviousValue != nil {
		previousValue = *caseEvent.PreviousValue
	}
	return models.CaseEvent{
		Id:             caseEvent.Id,
		CaseId:         caseEvent.CaseId,
		User:           user,
		CreatedAt:      caseEvent.CreatedAt,
		EventType:      models.CaseEventTypeFrom(caseEvent.EventType),
		AdditionalNote: additionalNote,
		ResourceId:     resourceId,
		ResourceType:   models.CaseEventResourceTypeFrom(resourceType),
		NewValue:       newValue,
		PreviousValue:  previousValue,
	}
}
