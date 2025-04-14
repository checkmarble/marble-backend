package models

import (
	"fmt"
	"slices"
	"time"
)

type Case struct {
	Id             string
	Contributors   []CaseContributor
	CreatedAt      time.Time
	Decisions      []DecisionWithRuleExecutions
	DecisionsCount int
	Events         []CaseEvent
	InboxId        string
	OrganizationId string
	AssignedTo     *UserId
	Name           string
	Status         CaseStatus
	Outcome        CaseOutcome
	Tags           []CaseTag
	Files          []CaseFile
	SnoozedUntil   *time.Time
	Boost          *BoostReason
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

func (c CaseStatus) IsFinalized() bool {
	return c == CaseClosed
}

type CaseMetadata struct {
	Id             string
	CreatedAt      time.Time
	OrganizationId string
	Status         CaseStatus
	InboxId        string
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
	DecisionIds    []string
	InboxId        string
	Name           string
	OrganizationId string
}

type UpdateCaseAttributes struct {
	Id      string
	InboxId string
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
	OrganizationId string
	Name           string
	StartDate      time.Time
	EndDate        time.Time
	Statuses       []CaseStatus
	InboxIds       []string
	IncludeSnoozed bool
}

type CaseListPage struct {
	Cases       []Case
	StartIndex  int
	EndIndex    int
	HasNextPage bool
}

type CaseWithRank struct {
	Case
	RankNumber int
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
