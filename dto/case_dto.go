package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APICase struct {
	Id        string         `json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Decisions []APIDecision  `json:"decisions"`
	Events    []APICaseEvent `json:"events"`
}

func AdaptCaseDto(c models.Case) APICase {
	apiCase := APICase{
		Id:        c.Id,
		CreatedAt: c.CreatedAt,
		Name:      c.Name,
		Status:    string(c.Status),
		Decisions: make([]APIDecision, len(c.Decisions)),
		Events:    make([]APICaseEvent, len(c.Events)),
	}

	for i, decision := range c.Decisions {
		apiCase.Decisions[i] = NewAPIDecision(decision)
	}
	for i, event := range c.Events {
		apiCase.Events[i] = NewAPICaseEvent(event)
	}

	return apiCase
}

type CreateCaseBody struct {
	Name        string   `json:"name" binding:"required"`
	DecisionIds []string `json:"decision_ids"`
}

type UpdateCaseBody struct {
	Name        string   `json:"name"`
	DecisionIds []string `json:"decision_ids"`
	Status      string   `json:"status"`
}

type CaseFilters struct {
	StartDate time.Time `form:"startDate" time_format`
	EndDate   time.Time `form:"endDate" time_format`
	Statuses  []string  `form:"statuses[]"`
}
