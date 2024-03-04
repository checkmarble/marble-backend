package models

import (
	"time"

	"github.com/guregu/null/v5"
)

type CaseEvent struct {
	Id             string
	CaseId         string
	UserId         null.String
	CreatedAt      time.Time
	EventType      CaseEventType
	AdditionalNote string
	ResourceId     string
	ResourceType   CaseEventResourceType
	NewValue       string
	PreviousValue  string
}

type CaseEventType string

const (
	CaseCreated              CaseEventType = "case_created"
	CaseCreatedAutomatically CaseEventType = "case_created_automatically"
	CaseStatusUpdated        CaseEventType = "status_updated"
	DecisionAdded            CaseEventType = "decision_added"
	CaseCommentAdded         CaseEventType = "comment_added"
	CaseNameUpdated          CaseEventType = "name_updated"
	CaseTagsUpdated          CaseEventType = "tags_updated"
	CaseFileAdded            CaseEventType = "file_added"
	CaseInboxChanged         CaseEventType = "inbox_changed"
)

type CaseEventResourceType string

const (
	DecisionResourceType CaseEventResourceType = "decision"
	CaseTagResourceType  CaseEventResourceType = "case_tag"
	CaseFileResourceType CaseEventResourceType = "case_file"
)

type CreateCaseEventAttributes struct {
	CaseId         string
	UserId         string
	EventType      CaseEventType
	AdditionalNote *string
	ResourceId     *string
	ResourceType   *CaseEventResourceType
	NewValue       *string
	PreviousValue  *string
}
