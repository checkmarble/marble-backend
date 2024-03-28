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
}

type APICaseWithDecisions struct {
	APICase
	Decisions []APIDecisionWithRules `json:"decisions"`
}

func AdaptCaseDto(c models.Case) APICase {
	return APICase{
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
}

func AdaptCaseWithDecisionsDto(c models.Case) APICaseWithDecisions {
	return APICaseWithDecisions{
		APICase: AdaptCaseDto(c),
		Decisions: pure_utils.Map(c.Decisions, func(d models.DecisionWithRuleExecutions) APIDecisionWithRules {
			return NewAPIDecisionWithRule(d, "")
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
	StartDate time.Time `form:"startDate" time_format`
	EndDate   time.Time `form:"endDate" time_format`
	Statuses  []string  `form:"statuses[]"`
	InboxIds  []string  `form:"inbox_ids[]"`
}
