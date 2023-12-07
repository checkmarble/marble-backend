package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
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
}

type APICaseWithDecisions struct {
	APICase
	Decisions []APIDecision `json:"decisions"`
}

func AdaptCaseDto(c models.Case) APICase {
	apiCase := APICase{
		Id:             c.Id,
		Contributors:   make([]APICaseContributor, len(c.Contributors)),
		CreatedAt:      c.CreatedAt,
		DecisionsCount: c.DecisionsCount,
		Events:         make([]APICaseEvent, len(c.Events)),
		InboxId:        c.InboxId,
		Name:           c.Name,
		Status:         string(c.Status),
		Tags:           make([]APICaseTag, len(c.Tags)),
	}

	for i, event := range c.Events {
		apiCase.Events[i] = NewAPICaseEvent(event)
	}
	for i, contributor := range c.Contributors {
		apiCase.Contributors[i] = NewAPICaseContributor(contributor)
	}
	for i, tag := range c.Tags {
		apiCase.Tags[i] = NewAPICaseTag(tag)
	}

	return apiCase
}

func AdaptCaseWithDecisionsDto(c models.Case) APICaseWithDecisions {
	return APICaseWithDecisions{
		APICase:   AdaptCaseDto(c),
		Decisions: make([]APIDecision, len(c.Decisions)),
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
