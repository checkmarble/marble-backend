package models

import (
	"time"
)

type CaseEvent struct {
	Id             string
	CaseId         string
	UserId         string
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
	CaseCreated       CaseEventType = "case_created"
	CaseStatusUpdated CaseEventType = "status_updated"
	DecisionAdded     CaseEventType = "decision_added"
	CaseCommentAdded  CaseEventType = "comment_added"
	CaseNameUpdated   CaseEventType = "name_updated"
	CaseTagAdded      CaseEventType = "tag_added"
	UnknownEvent      CaseEventType = "unknown_event"
)

type CaseEventResourceType string

const (
	DecisionResourceType CaseEventResourceType = "decision"
	CaseTagResourceType  CaseEventResourceType = "case_tag"
	UnknownResourceType  CaseEventResourceType = "unknown"
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
