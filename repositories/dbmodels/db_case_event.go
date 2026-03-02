package dbmodels

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/guregu/null/v5"
)

type DBCaseEvent struct {
	Id             string      `db:"id"`
	OrgId          uuid.UUID   `db:"org_id"`
	CaseId         string      `db:"case_id"`
	UserId         null.String `db:"user_id"`
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

// DBCaseCommentEvent is the result of a JOIN between case_events and entity_annotations,
// used specifically by ListCaseMixedCommentEvents.
type DBCaseCommentEvent struct {
	Id                string          `db:"id"`
	UserId            null.String     `db:"user_id"`
	CreatedAt         time.Time       `db:"created_at"`
	EventType         string          `db:"event_type"`
	AdditionalNote    *string         `db:"additional_note"`
	AnnotationPayload *json.RawMessage `db:"annotation_payload"`
}

func AdaptCaseCommentEvent(db DBCaseCommentEvent) (models.CaseCommentEvent, error) {
	event := models.CaseCommentEvent{
		Id:        db.Id,
		UserId:    db.UserId,
		CreatedAt: db.CreatedAt,
		Source:    models.CaseCommentSourceCase,
	}
	if db.AdditionalNote != nil {
		event.Comment = *db.AdditionalNote
	}

	if db.EventType == string(models.CaseEntityAnnotated) {
		event.Source = models.CaseCommentSourceEntity
		if db.AnnotationPayload != nil {
			var payload models.EntityAnnotationCommentPayload
			if err := json.Unmarshal(*db.AnnotationPayload, &payload); err == nil {
				event.Comment = payload.Text
			}
		}
	}

	return event, nil
}

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
		OrgId:          caseEvent.OrgId,
		CaseId:         caseEvent.CaseId,
		UserId:         caseEvent.UserId,
		CreatedAt:      caseEvent.CreatedAt,
		EventType:      models.CaseEventType(caseEvent.EventType),
		AdditionalNote: additionalNote,
		ResourceId:     resourceId,
		ResourceType:   models.CaseEventResourceType(resourceType),
		NewValue:       newValue,
		PreviousValue:  previousValue,
	}, nil
}
