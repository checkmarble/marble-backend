package params

import (
	"encoding/json"
	"time"

	gdto "github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/utils"
)

type ListDecisionsParams struct {
	pubapi.PaginationParams

	ScenarioId       *string         `form:"scenario_id" binding:"omitzero,uuid"`
	BatchExecutionId *string         `form:"batch_execution_id" binding:"omitzero,uuid"`
	CaseId           *string         `form:"case_id" binding:"omitzero,uuid"`
	Outcome          *string         `form:"outcome" binding:"omitzero,oneof=approve review block_and_review decline"`
	ReviewStatus     *string         `form:"review_status" binding:"omitzero,oneof=pending approve decline,excluded_unless=Outcome block_and_review"`
	TriggerObjectId  *string         `form:"trigger_object_id"`
	PivotValue       *string         `form:"pivot_value"`
	StartDate        pubapi.DateTime `form:"start"`
	EndDate          pubapi.DateTime `form:"end"`
}

func (p ListDecisionsParams) ToFilters() gdto.DecisionFilters {
	var filters gdto.DecisionFilters

	if !utils.NilOrZero(p.ScenarioId) {
		filters.ScenarioIds = []string{*p.ScenarioId}
	}
	if !utils.NilOrZero(p.BatchExecutionId) {
		filters.ScheduledExecutionIds = []string{*p.BatchExecutionId}
	}
	if !utils.NilOrZero(p.CaseId) {
		filters.CaseIds = []string{*p.CaseId}
	}
	if !utils.NilOrZero(p.Outcome) {
		filters.Outcomes = []string{*p.Outcome}
	}
	if !utils.NilOrZero(p.ReviewStatus) {
		filters.ReviewStatuses = []string{*p.ReviewStatus}
	}

	filters.TriggerObjectId = p.TriggerObjectId
	filters.PivotValue = p.PivotValue
	filters.StartDate = time.Time(p.StartDate)
	filters.EndDate = time.Time(p.EndDate)

	return filters
}

type CreateDecisionParams struct {
	ScenarioId    string          `json:"scenario_id" binding:"required,uuid"`
	TriggerObject json.RawMessage `json:"trigger_object" binding:"required"`
}

type CreateAllDecisionsParams struct {
	TriggerObjectType string          `json:"trigger_object_type" binding:"required"`
	TriggerObject     json.RawMessage `json:"trigger_object" binding:"required"`
}
