package params

import (
	"encoding/json"
	"time"

	gdto "github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pubapi"
	"github.com/checkmarble/marble-backend/utils"
)

const (
	DEFAULT_DECISIONS_DATE_RANGE = 30 * 24 * time.Hour
	// Should be a multiple of 24*time.Hour for errors to be formatted properly
	MAX_DECISIONS_DATE_RANGE = 90 * 24 * time.Hour
)

type ListDecisionsParams struct {
	pubapi.PaginationParams

	ScenarioId       *string `form:"scenario_id" binding:"omitzero,uuid"`
	BatchExecutionId *string `form:"batch_execution_id" binding:"omitzero,uuid"`
	CaseId           *string `form:"case_id" binding:"omitzero,uuid"`
	Outcome          *string `form:"outcome" binding:"omitzero,oneof=approve review block_and_review decline"`
	ReviewStatus     *string `form:"review_status" binding:"omitzero,oneof=pending approve decline,excluded_unless=Outcome block_and_review"`
	TriggerObjectId  *string `form:"trigger_object_id" binding:"omitzero,lte=256"`
	PivotValue       *string `form:"pivot_value" binding:"omitzero,lte=256"`

	// Both date filters are inclusive
	StartDate pubapi.DateTime `form:"start" binding:"required_with=EndDate"`
	EndDate   pubapi.DateTime `form:"end" binding:"required_with=StartDate"`
}

func (p ListDecisionsParams) ToFilters() gdto.DecisionFilters {
	now := time.Now()

	filters := gdto.DecisionFilters{
		StartDate:              now.Add(-DEFAULT_DECISIONS_DATE_RANGE),
		EndDate:                now,
		AllowInvalidScenarioId: true,
	}

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
	if !p.StartDate.IsZero() {
		filters.StartDate = time.Time(p.StartDate)
	}
	if !p.EndDate.IsZero() {
		filters.EndDate = time.Time(p.EndDate)
	}

	filters.AllowInvalidScenarioId = true
	filters.TriggerObjectId = p.TriggerObjectId
	filters.PivotValue = p.PivotValue

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
