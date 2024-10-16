package models

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
