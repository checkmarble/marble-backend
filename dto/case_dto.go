package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type APICase struct {
	Id             string               `json:"id"`
	Contributors   []APICaseContributor `json:"contributors"`
	CreatedAt      time.Time            `json:"created_at"`
	DecisionsCount int                  `json:"decisions_count"`
	Events         []APICaseEvent       `json:"events"`
	InboxId        uuid.UUID            `json:"inbox_id"`
	Name           string               `json:"name"`
	Status         string               `json:"status"`
	Outcome        string               `json:"outcome"`
	Tags           []APICaseTag         `json:"tags"`
	Files          []APICaseFile        `json:"files"`
	SnoozedUntil   *time.Time           `json:"snoozed_until,omitempty"`
	AssignedTo     *string              `json:"assigned_to,omitempty"`
	Boost          string               `json:"boost,omitempty"`
}

type APICaseWithDecisions struct {
	APICase
	Decisions []DecisionWithRules `json:"decisions"`
}

func AdaptCaseDto(c models.Case) APICase {
	dto := APICase{
		Id:             c.Id,
		Contributors:   pure_utils.Map(c.Contributors, NewAPICaseContributor),
		CreatedAt:      c.CreatedAt,
		DecisionsCount: c.DecisionsCount,
		Events:         pure_utils.Map(c.Events, NewAPICaseEvent),
		InboxId:        c.InboxId,
		Name:           c.Name,
		Status:         c.Status.EnrichedStatus(c.SnoozedUntil, c.Boost),
		Outcome:        string(c.Outcome),
		Tags:           pure_utils.Map(c.Tags, NewAPICaseTag),
		Files:          pure_utils.Map(c.Files, NewAPICaseFile),
		Boost:          c.Boost.String(),
	}

	if c.SnoozedUntil != nil && c.SnoozedUntil.After(time.Now()) {
		dto.SnoozedUntil = c.SnoozedUntil
	}
	if c.AssignedTo != nil {
		dto.AssignedTo = utils.Ptr(string(*c.AssignedTo))
	}

	return dto
}

type CastListPage struct {
	Items       []APICase `json:"items"`
	StartIndex  int       `json:"start_index"`
	EndIndex    int       `json:"end_index"`
	HasNextPage bool      `json:"has_next_page"`
}

func AdaptCaseListPage(casesPage models.CaseListPage) CastListPage {
	return CastListPage{
		Items:       pure_utils.Map(casesPage.Cases, AdaptCaseDto),
		StartIndex:  casesPage.StartIndex,
		EndIndex:    casesPage.EndIndex,
		HasNextPage: casesPage.HasNextPage,
	}
}

func AdaptCaseWithDecisionsDto(c models.Case) APICaseWithDecisions {
	return APICaseWithDecisions{
		APICase: AdaptCaseDto(c),
		Decisions: pure_utils.Map(c.Decisions, func(d models.DecisionWithRuleExecutions) DecisionWithRules {
			return NewDecisionWithRuleDto(d, nil, false)
		}),
	}
}

type CreateCaseBody struct {
	DecisionIds []string  `json:"decision_ids"`
	InboxId     uuid.UUID `json:"inbox_id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
}

type UpdateCaseBody struct {
	InboxId *uuid.UUID `json:"inbox_id"`
	Name    string     `json:"name"`
	Status  string     `json:"status"`
	Outcome string     `json:"outcome"`
}

type AddDecisionToCaseBody struct {
	DecisionIds []string `json:"decision_ids" binding:"required"`
}

type CreateCaseCommentBody struct {
	Comment string `json:"comment" binding:"required"`
}

// type UnmarshallingUuid struct {
// 	uuid.UUID
// }

// func (u *UnmarshallingUuid) UnmarshalParam(param string) error {
// 	parsed, err := uuid.Parse(param)
// 	if err != nil {
// 		return err
// 	}
// 	u.UUID = parsed
// 	return nil
// }

type CaseFilters struct {
	EndDate         time.Time     `form:"end_date"`
	InboxIds        []string      `form:"inbox_id[]"`
	StartDate       time.Time     `form:"start_date"`
	Statuses        []string      `form:"status[]"`
	Name            string        `form:"name"`
	IncludeSnoozed  bool          `form:"include_snoozed"`
	ExcludeAssigned bool          `form:"exclude_assigned"`
	AssigneeId      models.UserId `form:"assignee_id"`
}

func ParseSliceUUID(slice []string) ([]uuid.UUID, error) {
	parsed := make([]uuid.UUID, len(slice))
	for i, item := range slice {
		parsedItem, err := uuid.Parse(item)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse UUID in slice")
		}
		parsed[i] = parsedItem
	}
	return parsed, nil
}

func (f CaseFilters) Parse() (models.CaseFilters, error) {
	out := models.CaseFilters{
		EndDate:         f.EndDate,
		StartDate:       f.StartDate,
		Name:            f.Name,
		IncludeSnoozed:  f.IncludeSnoozed,
		ExcludeAssigned: f.ExcludeAssigned,
		AssigneeId:      f.AssigneeId,
	}

	var err error
	out.InboxIds, err = ParseSliceUUID(f.InboxIds)
	if err != nil {
		return out, errors.Wrap(err, "failed to parse inbox IDs")
	}

	statuses, err := models.ValidateCaseStatuses(f.Statuses)
	if err != nil {
		return out, err
	}
	out.Statuses = statuses

	return out, nil
}

type ReviewCaseDecisionsBody struct {
	DecisionId    string `json:"decision_id" binding:"required"`
	ReviewComment string `json:"review_comment"`
	ReviewStatus  string `json:"review_status" binding:"required"`
}

type CaseAssigneeDto struct {
	UserId models.UserId `json:"user_id"`
}

type CaseDecisionListDto struct {
	Decisions  []DecisionWithRules           `json:"decisions"`
	Pagination CaseDecisionListPaginationDto `json:"pagination"`
}

type CaseDecisionListPaginationDto struct {
	HasMore      bool   `json:"has_more"`
	NextCursorId string `json:"next_cursor_id"`
}
