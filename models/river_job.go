package models

import (
	"github.com/google/uuid"
)

// run async decision job
type AsyncDecisionArgs struct {
	DecisionToCreateId   string `json:"decision_to_create_id"`
	ObjectId             string `json:"object_id"`
	ScheduledExecutionId string `json:"scheduled_execution_id"`
	ScenarioIterationId  string `json:"scenario_iteration_id"`
}

func (AsyncDecisionArgs) Kind() string { return "async_decision" }

// job that starts with a scheduled execution and performs book keeping on the scheduled execution status
type ScheduledExecStatusSyncArgs struct {
	ScheduledExecutionId string `json:"scheduled_execution_id"`
}

func (ScheduledExecStatusSyncArgs) Kind() string { return "scheduled_execution_status_sync" }

type IndexCreationArgs struct {
	OrgId   uuid.UUID       `json:"org_id"`
	Indices []ConcreteIndex `json:"indices"`
}

func (IndexCreationArgs) Kind() string { return "index_creation" }

type IndexCreationStatusArgs struct {
	OrgId   uuid.UUID       `json:"org_id"`
	Indices []ConcreteIndex `json:"indices"`
}

func (IndexCreationStatusArgs) Kind() string { return "index_creation_status" }

type IndexCleanupArgs struct {
	OrgId uuid.UUID `json:"org_id"`
}

func (IndexCleanupArgs) Kind() string { return "index_cleanup" }

type IndexDeletionArgs struct {
	OrgId uuid.UUID `json:"org_id"`
}

func (IndexDeletionArgs) Kind() string { return "index_deletion" }

type TestRunSummaryArgs struct {
	OrgId uuid.UUID `json:"org_id"`
}

func (TestRunSummaryArgs) Kind() string { return "test_run_summary" }

type MatchEnrichmentArgs struct {
	OrgId       uuid.UUID `json:"org_id"`
	ScreeningId string    `json:"screening_id"`
}

func (MatchEnrichmentArgs) Kind() string { return "match_enrichment" }

type OffloadingArgs struct {
	OrgId uuid.UUID `json:"org_id"`
}

func (OffloadingArgs) Kind() string { return "offloading" }

type MetricsCollectionArgs struct{}

func (MetricsCollectionArgs) Kind() string { return "metrics_collection" }

type CaseReviewArgs struct {
	CaseId         uuid.UUID `json:"case_id"`
	AiCaseReviewId uuid.UUID `json:"ai_case_review_id"`
}

func (CaseReviewArgs) Kind() string { return "case_review" }

type AutoAssignmentArgs struct {
	OrgId   uuid.UUID `json:"org_id"`
	InboxId uuid.UUID `json:"inbox_id"`
}

func (AutoAssignmentArgs) Kind() string { return "auto_assignment" }

type DecisionWorkflowArgs struct {
	DecisionId string `json:"decision_id"`
}

func (DecisionWorkflowArgs) Kind() string { return "decision_workflow" }

type AnalyticsExportArgs struct {
	OrgId uuid.UUID `json:"org_id"`
}

func (AnalyticsExportArgs) Kind() string { return "analytics_export" }

type AnalyticsMergeArgs struct{}

func (AnalyticsMergeArgs) Kind() string { return "analytics_merge" }

type SendBillingEventArgs struct {
	Event BillingEvent `json:"event"`
}

func (SendBillingEventArgs) Kind() string { return "send_billing_event" }

type ContinuousScreeningDoScreeningArgs struct {
	ObjectType string    `json:"object_type"`
	OrgId      uuid.UUID `json:"org_id"`

	// Tell which action triggered the screening
	TriggerType ContinuousScreeningTriggerType `json:"trigger_type"`

	// MonitoringId is the ID from the object type specific monitoring table.
	MonitoringId uuid.UUID `json:"monitoring_id"`

	PreviousInternalId string `json:"previous_internal_id"`
	NewInternalId      string `json:"new_internal_id"`
}

func (ContinuousScreeningDoScreeningArgs) Kind() string {
	return "continuous_screening_do_screening"
}

type ContinuousScreeningEvaluateNeedArgs struct {
	OrgId      uuid.UUID `json:"org_id"`
	ObjectType string    `json:"object_type"`
	ObjectIds  []string  `json:"object_ids"`
}

func (ContinuousScreeningEvaluateNeedArgs) Kind() string {
	return "continuous_screening_evaluate_need"
}

type ContinuousScreeningScanDatasetUpdatesArgs struct{}

func (ContinuousScreeningScanDatasetUpdatesArgs) Kind() string {
	return "continuous_screening_scan_dataset_updates"
}

type ContinuousScreeningApplyDeltaFileArgs struct {
	OrgId    uuid.UUID `json:"org_id"`
	UpdateId uuid.UUID `json:"update_id"`
}

func (ContinuousScreeningApplyDeltaFileArgs) Kind() string {
	return "continuous_screening_apply_delta_file"
}

type ContinuousScreeningCreateFullDatasetArgs struct {
	OrgId string `json:"org_id"`
}

func (ContinuousScreeningCreateFullDatasetArgs) Kind() string {
	return "continuous_screening_create_full_dataset"
}
