package dbmodels

import (
	"marble/marble-backend/models"
	"time"
)

type DBScheduledExecution struct {
	Id                  string     `db:"id"`
	OrganizationId      string     `db:"organization_id"`
	ScenarioID          string     `db:"scenario_id"`
	ScenarioIterationID string     `db:"scenario_iteration_id"`
	Status              string     `db:"status"`
	StartedAt           time.Time  `db:"started_at"`
	FinishedAt          *time.Time `db:"finished_at"`
}

const TABLE_SCHEDULED_EXECUTIONS = "scheduled_executions"

var ScheduledExecutionFields = []string{"id", "organization_id", "scenario_id", "scenario_iteration_id", "status", "started_at", "finished_at"}

func AdaptScheduledExecution(db DBScheduledExecution) models.ScheduledExecution {
	return models.ScheduledExecution{
		ID:                  db.Id,
		ScenarioIterationID: db.ScenarioIterationID,
		Status:              db.Status,
		StartedAt:           db.StartedAt,
		FinishedAt:          db.FinishedAt,
	}
}

type UpdateScheduledExecutionDbBody struct {
	Status *string
}
