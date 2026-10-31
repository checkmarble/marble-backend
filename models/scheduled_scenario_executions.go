package models

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ScheduledExecution struct {
	Id                         string
	OrganizationId             uuid.UUID
	ScenarioId                 string
	ScenarioIterationId        string
	ScenarioVersion            string
	Status                     ScheduledExecutionStatus
	StartedAt                  time.Time
	FinishedAt                 *time.Time
	NumberOfCreatedDecisions   int
	NumberOfEvaluatedDecisions int
	NumberOfPlannedDecisions   *int
	Scenario                   Scenario
	Manual                     bool

	// Set when the execution is processed from an object-id manifest in blob storage. Nil/zero
	// when it is processed with one decision row and job per object.
	ManifestBlobKey       *string
	ManifestByteOffset    int64
	ManifestRowsProcessed int64
	Deadline              *time.Time

	// DeduplicateObjects is the effective dedup setting for this run, resolved once at
	// creation (see CreateScheduledExecutionInput.DeduplicateObjects) from
	// Scenario.DeduplicateBatchObjects. Deliberately snapshotted rather than read live from
	// the scenario, since a run can span several River slices and the setting must not
	// change mid-run (see the scheduled_execution_deduplicate migration).
	DeduplicateObjects bool

	// Set when a cancellation is requested. CancelDeadline is the latest time at which the job
	// owning the execution may still be running: past it, a `cancelling` execution is considered
	// stopped and is finalized as `cancelled` without the worker's acknowledgement.
	CancelRequestedAt *time.Time
	CancelDeadline    *time.Time
}

type PaginatedScheduledExecutions struct {
	Executions []ScheduledExecution
	HasMore    bool
}

type ScheduledExecutionStatus int

const (
	ScheduledExecutionPending ScheduledExecutionStatus = iota
	ScheduledExecutionProcessing
	ScheduledExecutionSuccess
	ScheduledExecutionPartialFailure
	ScheduledExecutionFailure
	// A cancellation was requested but the worker may still be running. Still counts as active,
	// so that no other execution of the scenario can start until it is actually stopped.
	ScheduledExecutionCancelling
	ScheduledExecutionCancelled
)

// EffectiveScheduledExecutionStatus returns the status an execution should be considered in. A
// cancelling execution whose cancel deadline has passed is reported as cancelled: the worker that
// owned it cannot be running anymore, so it must not keep blocking the scenario, even if no job
// has finalized it in the database yet.
func EffectiveScheduledExecutionStatus(status ScheduledExecutionStatus, cancelDeadline *time.Time, now time.Time) ScheduledExecutionStatus {
	if status == ScheduledExecutionCancelling && cancelDeadline != nil && now.After(*cancelDeadline) {
		return ScheduledExecutionCancelled
	}

	return status
}

// ErrScheduledExecutionNotProcessing is returned when a write expected the execution to be in
// the processing state but it was not (typically because a cancellation was requested).
var ErrScheduledExecutionNotProcessing = errors.New("scheduled execution is not processing")

func (s ScheduledExecutionStatus) IsActive() bool {
	switch s {
	case ScheduledExecutionPending, ScheduledExecutionProcessing, ScheduledExecutionCancelling:
		return true
	}
	return false
}

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
	case ScheduledExecutionCancelling:
		return "cancelling"
	case ScheduledExecutionCancelled:
		return "cancelled"
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
	case "cancelling":
		return ScheduledExecutionCancelling
	case "cancelled":
		return ScheduledExecutionCancelled
	}
	return ScheduledExecutionPending
}

type UpdateScheduledExecutionStatusInput struct {
	Id     string
	Status ScheduledExecutionStatus
	// When set, the update only applies if the execution currently has this status. Otherwise it
	// applies from any status that is not final and not being cancelled (pending or processing).
	// This simulates a compare-and-swap operation.
	FromStatus                 *ScheduledExecutionStatus
	NumberOfCreatedDecisions   *int
	NumberOfEvaluatedDecisions *int
}

type UpdateScheduledExecutionInput struct {
	Id                       string
	NumberOfPlannedDecisions *int
	ManifestBlobKey          *string
	Deadline                 *time.Time
}

// AdvanceScheduledExecutionManifestInput records progress of the v2 coordinator after a
// batch is persisted: the manifest cursor and the running totals, written atomically with
// the decision inserts so a crash resumes exactly where it left off. Values are absolute
// (the coordinator is the single writer and tracks cumulative counts).
type AdvanceScheduledExecutionManifestInput struct {
	Id                         string
	ManifestByteOffset         int64
	ManifestRowsProcessed      int64
	NumberOfCreatedDecisions   int
	NumberOfEvaluatedDecisions int
}

type CreateScheduledExecutionInput struct {
	OrganizationId      uuid.UUID
	ScenarioId          string
	ScenarioIterationId string
	Manual              bool

	// DeduplicateObjects has two roles depending on which layer builds this struct:
	//   - As the usecase's incoming parameter (from the API body): nil means "no override,
	//     use the scenario's default"; a non-nil value overrides it for this run only.
	//   - As the payload the usecase then builds for the repository: always resolved to a
	//     definite value (nil override folded into Scenario.DeduplicateBatchObjects) before
	//     the call, so the value snapshotted on scheduled_executions is never ambiguous.
	// The repository still defaults a stray nil to false (see CreateScheduledExecution) as a
	// defensive fallback, not as part of the intended resolution path.
	DeduplicateObjects *bool
}

type FinalizeExpiredCancellationsFilters struct {
	ScheduledExecutionId string
	ScenarioId           string
}

type ListScheduledExecutionsFilters struct {
	OrganizationId uuid.UUID
	ScenarioId     string
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

	if ast.IsMathOperation(f.Operator) {
		attrs, err := f.Operator.Attributes()
		if err != nil {
			// Should never happen because we check for math operations above
			return "", nil
		}
		// apply NULLIF to protect against division by zero
		switch f.Operator {
		case ast.FUNC_DIVIDE:
			sql = fmt.Sprintf("%s %s NULLIF(%s, 0)",
				left, attrs.AstName, right)
		case ast.FUNC_NOT_EQUAL:
			// AstName for NOT_EQUAL is "≠" (Unicode), which is not valid SQL
			sql = fmt.Sprintf("%s <> %s", left, right)
		default:
			sql = fmt.Sprintf("%s %s %s", left,
				attrs.AstName, right)
		}
	} else if ast.IsStringComparison(f.Operator) {
		sql = fmt.Sprintf("%s ILIKE CONCAT('%%',%s::text,'%%')", left, right)
	} else if ast.IsInListComparison(f.Operator) {
		sql = fmt.Sprintf("%s = ANY(%s)", left, right)
	} else if f.Operator == ast.FUNC_IS_EMPTY {
		sql = fmt.Sprintf("(%s IS NULL OR %s = '')", left, left)
	} else if f.Operator == ast.FUNC_IS_NOT_EMPTY {
		sql = fmt.Sprintf("(%s IS NOT NULL AND %s != '')", left, left)
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
