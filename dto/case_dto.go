package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type APICase struct {
	Id             string               `json:"id"`
	Contributors   []APICaseContributor `json:"contributors"`
	CreatedAt      time.Time            `json:"created_at"`
	DecisionsCount int                  `json:"decisions_count"`
	Events         []APICaseEvent       `json:"events"`
	InboxId        string               `json:"inbox_id"`
	Name           string               `json:"name"`
	Status         string               `json:"status"`
	Tags           []APICaseTag         `json:"tags"`
	Files          []APICaseFile        `json:"files"`
	SnoozedUntil   *time.Time           `json:"snoozed_until,omitempty"`
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
		Status:         string(c.Status),
		Tags:           pure_utils.Map(c.Tags, NewAPICaseTag),
		Files:          pure_utils.Map(c.Files, NewAPICaseFile),
	}

	if c.SnoozedUntil != nil && c.SnoozedUntil.After(time.Now()) {
		dto.SnoozedUntil = c.SnoozedUntil
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
	DecisionIds []string `json:"decision_ids"`
	InboxId     string   `json:"inbox_id" binding:"required"`
	Name        string   `json:"name" binding:"required"`
}

type UpdateCaseBody struct {
	InboxId string `json:"inbox_id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
}

type AddDecisionToCaseBody struct {
	DecisionIds []string `json:"decision_ids" binding:"required"`
}

type CreateCaseCommentBody struct {
	Comment string `json:"comment" binding:"required"`
}

type CaseFilters struct {
	EndDate        time.Time `form:"end_date"`
	InboxIds       []string  `form:"inbox_id[]"`
	StartDate      time.Time `form:"start_date"`
	Statuses       []string  `form:"status[]"`
	Name           string    `form:"name"`
	IncludeSnoozed bool      `form:"include_snoozed"`
}

type ReviewCaseDecisionsBody struct {
	DecisionId    string `json:"decision_id" binding:"required"`
	ReviewComment string `json:"review_comment" binding:"required"`
	ReviewStatus  string `json:"review_status" binding:"required"`
}

type CaseAssigneeDto struct {
	UserId models.UserId `json:"user_id"`
}
