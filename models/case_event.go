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
	CaseCommentAdded      CaseEventType = "comment_added"
	CaseCreated           CaseEventType = "case_created"
	CaseFileAdded         CaseEventType = "file_added"
	CaseInboxChanged      CaseEventType = "inbox_changed"
	CaseNameUpdated       CaseEventType = "name_updated"
	CaseRuleSnoozeCreated CaseEventType = "rule_snooze_created"
	CaseStatusUpdated     CaseEventType = "status_updated"
	CaseOutcomeUpdated    CaseEventType = "outcome_updated"
	CaseTagsUpdated       CaseEventType = "tags_updated"
	CaseAssigned          CaseEventType = "case_assigned"
	SarCreated            CaseEventType = "sar_created"
	SarDeleted            CaseEventType = "sar_deleted"
	SarStatusChanged      CaseEventType = "sar_status_changed"
	SarFileUploaded       CaseEventType = "sar_file_uploaded"
	SanctionCheckReviewed CaseEventType = "sanction_check_reviewed"
	DecisionAdded         CaseEventType = "decision_added"
	DecisionReviewed      CaseEventType = "decision_reviewed"
	CaseSnoozed           CaseEventType = "case_snoozed"
	CaseUnsnoozed         CaseEventType = "case_unsnoozed"
)

type CaseEventResourceType string

const (
	DecisionResourceType   CaseEventResourceType = "decision"
	CaseTagResourceType    CaseEventResourceType = "case_tag"
	CaseFileResourceType   CaseEventResourceType = "case_file"
	RuleSnoozeResourceType CaseEventResourceType = "rule_snooze"
	SarResourceType        CaseEventResourceType = "sar"
)

type CreateCaseEventAttributes struct {
	CaseId         string
	UserId         *string
	EventType      CaseEventType
	AdditionalNote *string
	ResourceId     *string
	ResourceType   *CaseEventResourceType
	NewValue       *string
	PreviousValue  *string
}
