package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type ScheduledExecutionDto struct {
	Id                         string     `json:"id"`
	ScenarioIterationId        string     `json:"scenario_iteration_id"`
	Status                     string     `json:"status"`
	StartedAt                  time.Time  `json:"started_at"`
	FinishedAt                 *time.Time `json:"finished_at"`
	NumberOfCreatedDecisions   int        `json:"number_of_created_decisions"`
	NumberOfEvaluatedDecisions int        `json:"number_of_evaluated_decisions"`
	NumberOfPlannedDecisions   *int       `json:"number_of_planned_decisions"`
	ScenarioId                 string     `json:"scenario_id"`
	ScenarioName               string     `json:"scenario_name"`
	ScenarioTriggerObjectType  string     `json:"scenario_trigger_object_type"`
	Manual                     bool       `json:"manual"`

	// ManifestRowsProcessed counts manifest rows the v2 batch coordinator has consumed so
	// far -- evaluated *or* skipped by dedup -- out of NumberOfPlannedDecisions. It only
	// advances for v2 (manifest-based) executions; it stays 0 for v1 (per-object row+job)
	// executions, where NumberOfEvaluatedDecisions already plays that role.
	//
	// Exposed as a separate progress signal because NumberOfEvaluatedDecisions is not
	// reliable for that purpose once Scenario.DeduplicateBatchObjects is enabled: objects
	// already scored by a previous run are dropped by the pre-filter *before* evaluation
	// (see FilterAlreadyScoredObjects in the coordinator), so NumberOfEvaluatedDecisions can
	// stay frozen at 0 for an entire run even though the coordinator is actively working
	// through the manifest -- from that field alone, indistinguishable from a stuck run.
	// ManifestRowsProcessed still advances every batch regardless of dedup, since it counts
	// consumed manifest rows, not evaluations.
	ManifestRowsProcessed int64 `json:"manifest_rows_processed"`

	// DeduplicateObjects is the effective, snapshotted dedup setting for this specific run
	// (see models.ScheduledExecution.DeduplicateObjects) -- not the scenario's current
	// setting, which may have changed since. Exposed so a "why did this run create 0
	// decisions" question can be answered from the execution alone.
	DeduplicateObjects bool `json:"deduplicate_objects"`
}

func AdaptScheduledExecutionDto(ExecutionBatch models.ScheduledExecution) ScheduledExecutionDto {
	return ScheduledExecutionDto{
		Id:                         ExecutionBatch.Id,
		ScenarioIterationId:        ExecutionBatch.ScenarioIterationId,
		Status:                     ExecutionBatch.Status.String(),
		StartedAt:                  ExecutionBatch.StartedAt,
		FinishedAt:                 ExecutionBatch.FinishedAt,
		NumberOfCreatedDecisions:   ExecutionBatch.NumberOfCreatedDecisions,
		NumberOfEvaluatedDecisions: ExecutionBatch.NumberOfEvaluatedDecisions,
		NumberOfPlannedDecisions:   ExecutionBatch.NumberOfPlannedDecisions,
		ScenarioId:                 ExecutionBatch.ScenarioId,
		ScenarioName:               ExecutionBatch.Scenario.Name,
		ScenarioTriggerObjectType:  ExecutionBatch.Scenario.TriggerObjectType,
		Manual:                     ExecutionBatch.Manual,
		ManifestRowsProcessed:      ExecutionBatch.ManifestRowsProcessed,
		DeduplicateObjects:         ExecutionBatch.DeduplicateObjects,
	}
}

// CreateScheduledExecutionBody is optional: this endpoint historically took no body, so an
// empty payload must remain valid (see handleCreateScheduledExecution).
type CreateScheduledExecutionBody struct {
	// DeduplicateObjects overrides the scenario's default for this run only. Nil means no
	// override: the scenario's persistent setting applies. This is the escape hatch to force
	// a re-score after fixing a faulty trigger condition, without a support-side DB fixup.
	DeduplicateObjects *bool `json:"deduplicate_objects"`
}
