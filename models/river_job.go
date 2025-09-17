package models

import "github.com/google/uuid"

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
	OrgId   string          `json:"org_id"`
	Indices []ConcreteIndex `json:"indices"`
}

func (IndexCreationArgs) Kind() string { return "index_creation" }

type IndexCreationStatusArgs struct {
	OrgId   string          `json:"org_id"`
	Indices []ConcreteIndex `json:"indices"`
}

func (IndexCreationStatusArgs) Kind() string { return "index_creation_status" }

type IndexCleanupArgs struct {
	OrgId string `json:"org_id"`
}

func (IndexCleanupArgs) Kind() string { return "index_cleanup" }

type IndexDeletionArgs struct {
	OrgId string `json:"org_id"`
}

func (IndexDeletionArgs) Kind() string { return "index_deletion" }

type TestRunSummaryArgs struct {
	OrgId string `json:"org_id"`
}

func (TestRunSummaryArgs) Kind() string { return "test_run_summary" }

type MatchEnrichmentArgs struct {
	OrgId                  string `json:"org_id"`
	SanctionCheckId_deprec string `json:"sanction_check_id"` //nolint:tagliatelle
	ScreeningId            string `json:"screening_id"`
}

func (MatchEnrichmentArgs) Kind() string { return "match_enrichment" }

type OffloadingArgs struct {
	OrgId string `json:"org_id"`
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
	OrgId   string    `json:"org_id"`
	InboxId uuid.UUID `json:"inbox_id"`
}

func (AutoAssignmentArgs) Kind() string { return "auto_assignment" }

type DecisionWorkflowArgs struct {
	DecisionId string `json:"decision_id"`
}

func (DecisionWorkflowArgs) Kind() string { return "decision_workflow" }

type AnalyticsExportArgs struct {
	OrgId string `json:"org_id"`
}

func (AnalyticsExportArgs) Kind() string { return "analytics_export" }
