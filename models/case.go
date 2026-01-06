package models

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

type CaseType int

const (
	CaseTypeUnknown CaseType = iota
	CaseTypeDecision
	CaseTypeContinuousScreening
)

func (t CaseType) String() string {
	switch t {
	case CaseTypeDecision:
		return "decision"
	case CaseTypeContinuousScreening:
		return "continuous_screening"
	default:
		return "unknown"
	}
}

func CaseTypeFromString(s string) CaseType {
	switch s {
	case "decision":
		return CaseTypeDecision
	case "continuous_screening":
		return CaseTypeContinuousScreening
	default:
		return CaseTypeUnknown
	}
}

type Case struct {
	Id                   string
	Contributors         []CaseContributor
	CreatedAt            time.Time
	Decisions            []Decision
	DecisionsCount       int
	ContinuousScreenings []ContinuousScreeningWithMatches
	Events               []CaseEvent
	InboxId              uuid.UUID
	OrganizationId       uuid.UUID
	AssignedTo           *UserId
	Name                 string
	Status               CaseStatus
	Outcome              CaseOutcome
	Tags                 []CaseTag
	Files                []CaseFile
	SnoozedUntil         *time.Time
	Boost                *BoostReason
	Type                 CaseType
}

type CaseReferents struct {
	Id       string
	Inbox    Inbox
	Assignee *User
}

func (c Case) GetMetadata() CaseMetadata {
	return CaseMetadata{
		Id:             c.Id,
		CreatedAt:      c.CreatedAt,
		OrganizationId: c.OrganizationId,
		Status:         c.Status,
		InboxId:        c.InboxId,
	}
}

func (c Case) IsSnoozed() bool {
	return c.SnoozedUntil != nil && c.SnoozedUntil.After(time.Now())
}

func (s CaseStatus) IsFinalized() bool {
	return s == CaseClosed
}

type CaseMetadata struct {
	Id             string
	CreatedAt      time.Time
	OrganizationId uuid.UUID
	Status         CaseStatus
	InboxId        uuid.UUID
	Outcome        CaseOutcome
}

type CaseStatus string

const (
	CasePending       CaseStatus = "pending"
	CaseInvestigating CaseStatus = "investigating"
	CaseClosed        CaseStatus = "closed"
	CaseUnknownStatus CaseStatus = "unknown"
)

func (s CaseStatus) CanTransition(newStatus CaseStatus) bool {
	if s == newStatus {
		return true
	}

	switch s {
	case CasePending:
		return true
	case CaseInvestigating:
		return slices.Contains([]CaseStatus{CaseClosed}, newStatus)
	case CaseClosed:
		return slices.Contains([]CaseStatus{CaseInvestigating}, newStatus)
	default:
		return false
	}
}

func (s CaseStatus) EnrichedStatus(snoozedUntil *time.Time, boost *BoostReason) string {
	if (s == CaseInvestigating || s == CasePending) && snoozedUntil != nil && snoozedUntil.After(time.Now()) {
		return "snoozed"
	}
	if s == CaseInvestigating && boost != nil {
		return "waiting_for_action"
	}
	return string(s)
}

type CaseOutcome string

const (
	CaseOutcomeUnset   = "unset"
	CaseConfirmedRisk  = "confirmed_risk"
	CaseValuableAlert  = "valuable_alert"
	CaseFalsePositive  = "false_positive"
	CaseUnknownOutcome = "unknown"
)

var ValidCaseOutcomes = []CaseOutcome{CaseOutcomeUnset, CaseConfirmedRisk, CaseValuableAlert, CaseFalsePositive}

type CreateCaseAttributes struct {
	DecisionIds            []string
	ContinuousScreeningIds []uuid.UUID
	InboxId                uuid.UUID
	Name                   string
	OrganizationId         uuid.UUID
	AssigneeId             *string
	Type                   CaseType
}

type UpdateCaseAttributes struct {
	Id      string
	InboxId *uuid.UUID
	Name    string
	Status  CaseStatus
	Outcome CaseOutcome
	Boost   BoostReason
}

type CreateCaseCommentAttributes struct {
	Id      string
	Comment string
}

type CaseFilters struct {
	OrganizationId  uuid.UUID
	Name            string
	StartDate       time.Time
	EndDate         time.Time
	Statuses        []CaseStatus
	InboxIds        []uuid.UUID
	IncludeSnoozed  bool
	ExcludeAssigned bool
	AssigneeId      UserId
	TagId           *uuid.UUID

	UseLinearOrdering bool
}

const (
	DEFAULT_SIMILARITY_THRESHOLD = 0.5
)

type CaseListPage struct {
	Cases       []Case
	HasNextPage bool
}

const CasesSortingCreatedAt = SortingFieldCreatedAt

func ValidateCaseStatuses(statuses []string) ([]CaseStatus, error) {
	sanitizedStatuses := make([]CaseStatus, len(statuses))
	for i, status := range statuses {
		sanitizedStatuses[i] = CaseStatus(status)
		if sanitizedStatuses[i] == CaseUnknownStatus {
			return []CaseStatus{}, fmt.Errorf("invalid status: %s %w", status, BadParameterError)
		}
	}
	return sanitizedStatuses, nil
}

type ReviewCaseDecisionsBody struct {
	DecisionId    string
	ReviewComment string
	ReviewStatus  string
	UserId        string
}

type CaseSnoozeRequest struct {
	UserId UserId
	CaseId string
	Until  time.Time
}

type CaseAssignementRequest struct {
	UserId     UserId
	CaseId     string
	AssigneeId *UserId
}

type BoostReason string

const (
	BoostUnboost     BoostReason = ""
	BoostUnsnoozed   BoostReason = "unsnoozed"
	BoostReassigned  BoostReason = "reassigned"
	BoostEscalated   BoostReason = "escalated"
	BoostNewDecision BoostReason = "new_decision"
)

func (br *BoostReason) String() string {
	if br == nil {
		return ""
	}
	return string(*br)
}

const CaseDecisionsPerPage = 50

type CaseDecisionsRequest struct {
	OrgId    string
	CaseId   string
	CursorId string
	Limit    int
}

type CaseMassUpdateAction string

const (
	CaseMassUpdateClose       CaseMassUpdateAction = "close"
	CaseMassUpdateReopen      CaseMassUpdateAction = "reopen"
	CaseMassUpdateAssign      CaseMassUpdateAction = "assign"
	CaseMassUpdateMoveToInbox CaseMassUpdateAction = "move_to_inbox"
	CaseMassUpdateUnknown     CaseMassUpdateAction = "unknown"
)

func (a CaseMassUpdateAction) String() string {
	return string(a)
}

func CaseMassUpdateActionFromString(s string) CaseMassUpdateAction {
	switch s {
	case "close":
		return CaseMassUpdateClose
	case "reopen":
		return CaseMassUpdateReopen
	case "assign":
		return CaseMassUpdateAssign
	case "move_to_inbox":
		return CaseMassUpdateMoveToInbox
	default:
		return CaseMassUpdateUnknown
	}
}
