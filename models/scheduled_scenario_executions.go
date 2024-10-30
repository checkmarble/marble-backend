package models

import (
	"fmt"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
)

type ScheduledExecution struct {
	Id                         string
	OrganizationId             string
	ScenarioId                 string
	ScenarioIterationId        string
	Status                     ScheduledExecutionStatus
	StartedAt                  time.Time
	FinishedAt                 *time.Time
	NumberOfCreatedDecisions   int
	NumberOfEvaluatedDecisions int
	NumberOfPlannedDecisions   *int
	Scenario                   Scenario
	Manual                     bool
}

type ScheduledExecutionStatus int

const (
	ScheduledExecutionPending ScheduledExecutionStatus = iota
	ScheduledExecutionProcessing
	ScheduledExecutionSuccess
	ScheduledExecutionPartialFailure
	ScheduledExecutionFailure
)

func (s ScheduledExecutionStatus) String() string {
	switch s {
	case ScheduledExecutionPending:
		return "pending"
	case ScheduledExecutionProcessing:
		return "processing"
	case ScheduledExecutionSuccess:
		return "success"
	case ScheduledExecutionPartialFailure:
		return "partial_failure"
	case ScheduledExecutionFailure:
		return "failure"
	}
	return "pending"
}

func ScheduledExecutionStatusFrom(s string) ScheduledExecutionStatus {
	switch s {
	case "pending":
		return ScheduledExecutionPending
	case "success":
		return ScheduledExecutionSuccess
	case "failure":
		return ScheduledExecutionFailure
	case "partial_failure":
		return ScheduledExecutionPartialFailure
	case "processing":
		return ScheduledExecutionProcessing
	}
	return ScheduledExecutionPending
}

type UpdateScheduledExecutionStatusInput struct {
	Id                         string
	Status                     ScheduledExecutionStatus
	NumberOfCreatedDecisions   *int
	NumberOfEvaluatedDecisions *int
	CurrentStatusCondition     ScheduledExecutionStatus // Used for optimistic locking
}

type UpdateScheduledExecutionInput struct {
	Id                       string
	NumberOfPlannedDecisions *int
}

type CreateScheduledExecutionInput struct {
	OrganizationId      string
	ScenarioId          string
	ScenarioIterationId string
	Manual              bool
}

type ListScheduledExecutionsFilters struct {
	OrganizationId string
	ScenarioId     string
	Status         []ScheduledExecutionStatus
	ExcludeManual  bool
}

type Filter struct {
	LeftSql           string
	LeftValue         any
	LeftNestedFilter  *Filter
	Operator          ast.Function
	RightSql          string
	RightValue        any
	RightNestedFilter *Filter
}

func mathComparisonFuncToString(f ast.Function) string {
	switch f {
	case ast.FUNC_GREATER:
		return ">"
	case ast.FUNC_GREATER_OR_EQUAL:
		return ">="
	case ast.FUNC_LESS:
		return "<"
	case ast.FUNC_LESS_OR_EQUAL:
		return "<="
	case ast.FUNC_EQUAL:
		return "="
	case ast.FUNC_NOT_EQUAL:
		return "!="
	case ast.FUNC_ADD:
		return "+"
	case ast.FUNC_SUBTRACT:
		return "-"
	case ast.FUNC_MULTIPLY:
		return "*"
	case ast.FUNC_DIVIDE:
		return "/"
	default:
		return ""
	}
}

func isMathOperation(f ast.Function) bool {
	return slices.Contains([]ast.Function{
		ast.FUNC_GREATER,
		ast.FUNC_GREATER_OR_EQUAL,
		ast.FUNC_LESS,
		ast.FUNC_LESS_OR_EQUAL,
		ast.FUNC_EQUAL,
		ast.FUNC_NOT_EQUAL,
		ast.FUNC_ADD,
		ast.FUNC_SUBTRACT,
		ast.FUNC_MULTIPLY,
		ast.FUNC_DIVIDE,
	}, f)
}

func isStringComparison(f ast.Function) bool {
	return slices.Contains([]ast.Function{
		ast.FUNC_STRING_CONTAINS,
		ast.FUNC_STRING_NOT_CONTAIN,
	}, f)
}

func isInListComparison(f ast.Function) bool {
	return slices.Contains([]ast.Function{
		ast.FUNC_IS_IN_LIST,
		ast.FUNC_IS_NOT_IN_LIST,
	}, f)
}

// Disclaimer: this logic creates some coupling between the models and SQL queries. It's not great, but I could not find
// a simple enough abstraction to avoid doing this.
func (f Filter) ToSql() (sql string, args []any) {
	var left string
	if f.LeftSql != "" {
		left = f.LeftSql
	} else if f.LeftValue != nil {
		left = "?"
		args = append(args, f.LeftValue)
	} else if f.LeftNestedFilter != nil {
		leftSql, leftArgs := f.LeftNestedFilter.ToSql()
		left = fmt.Sprintf("(%s)", leftSql)
		args = append(args, leftArgs...)
	}

	var right string
	if f.RightSql != "" {
		right = f.RightSql
	} else if f.RightValue != nil {
		right = "?"
		args = append(args, f.RightValue)
	} else if f.RightNestedFilter != nil {
		rightSql, rightArgs := f.RightNestedFilter.ToSql()
		right = fmt.Sprintf("(%s)", rightSql)
		args = append(args, rightArgs...)
	}

	if isMathOperation(f.Operator) {
		// apply NULLIF to protect against division by zero
		if f.Operator == ast.FUNC_DIVIDE {
			sql = fmt.Sprintf("%s %s NULLIF(%s, 0)",
				left, mathComparisonFuncToString(f.Operator), right)
		} else {
			sql = fmt.Sprintf("%s %s %s", left,
				mathComparisonFuncToString(f.Operator), right)
		}
	} else if isStringComparison(f.Operator) {
		sql = fmt.Sprintf("%s ILIKE CONCAT('%%',%s::text,'%%')", left, right)
	} else if isInListComparison(f.Operator) {
		sql = fmt.Sprintf("%s = ANY(%s)", left, right)
	}

	return sql, args
}

type TableIdentifier struct {
	Schema string
	Table  string
}

type DecisionToCreateStatus string

type DecisionToCreate struct {
	Id                   string
	ScheduledExecutionId string
	ObjectId             string
	Status               DecisionToCreateStatus
	CreatedAt            time.Time
	UpdateAt             time.Time
}

type DecisionToCreateBatchCreateInput struct {
	ScheduledExecutionId string
	ObjectId             []string
}

const (
	DecisionToCreateStatusPending                  = "pending"
	DecisionToCreateStatusCreated                  = "created"
	DecisionToCreateStatusFailed                   = "failed"
	DecisionToCreateStatusTriggerConditionMismatch = "trigger_mismatch"
)

type ListDecisionsToCreateFilters struct {
	ScheduledExecutionId string
	Status               []DecisionToCreateStatus
}

type DecisionToCreateCountMetadata struct {
	Created                  int
	TriggerConditionMismatch int
	SuccessfullyEvaluated    int
}
