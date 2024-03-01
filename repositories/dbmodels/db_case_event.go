package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/jackc/pgx/v5/pgtype"
)

type DBCaseEvent struct {
	Id             string      `db:"id"`
	CaseId         string      `db:"case_id"`
	UserId         pgtype.Text `db:"user_id"`
	CreatedAt      time.Time   `db:"created_at"`
	EventType      string      `db:"event_type"`
	AdditionalNote *string     `db:"additional_note"`
	ResourceId     *string     `db:"resource_id"`
	ResourceType   *string     `db:"resource_type"`
	NewValue       *string     `db:"new_value"`
	PreviousValue  *string     `db:"previous_value"`
}

const TABLE_CASE_EVENTS = "case_events"

var SelectCaseEventColumn = utils.ColumnList[DBCaseEvent]()

func AdaptCaseEvent(caseEvent DBCaseEvent) (models.CaseEvent, error) {
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
		UserId:         caseEvent.UserId.String,
		CreatedAt:      caseEvent.CreatedAt,
		EventType:      models.CaseEventType(caseEvent.EventType),
		AdditionalNote: additionalNote,
		ResourceId:     resourceId,
		ResourceType:   models.CaseEventResourceType(resourceType),
		NewValue:       newValue,
		PreviousValue:  previousValue,
	}, nil
}
