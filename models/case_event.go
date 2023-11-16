package models

import (
	"time"
)

type CaseEvent struct {
	Id             string
	CaseId         string
	User           User
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
	CaseCreated   CaseEventType = "case_created"
	StatusUpdated CaseEventType = "status_updated"
	DecisionAdded CaseEventType = "decision_added"
	CommentAdded  CaseEventType = "comment_added"
	UnknownEvent  CaseEventType = "unknown_event"
)

type CaseEventResourceType string

const (
	DecisionResourceType CaseEventResourceType = "decision"
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
