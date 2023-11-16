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

func CaseEventTypeFrom(s string) CaseEventType {
	switch s {
	case "case_created":
		return CaseCreated
	case "status_updated":
		return StatusUpdated
	case "decision_added":
		return DecisionAdded
	case "comment_added":
		return CommentAdded
	default:
		return UnknownEvent
	}
}

type CaseEventResourceType string

const (
	DecisionResourceType CaseEventResourceType = "decision"
	UnknownResourceType  CaseEventResourceType = "unknown"
)

func CaseEventResourceTypeFrom(s string) CaseEventResourceType {
	switch s {
	case "decision":
		return DecisionResourceType
	default:
		return UnknownResourceType
	}
}

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
