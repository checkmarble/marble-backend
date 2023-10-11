package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type DBScheduledExecution struct {
	Id                       string     `db:"id"`
	OrganizationId           string     `db:"organization_id"`
	ScenarioId               string     `db:"scenario_id"`
	ScenarioIterationId      string     `db:"scenario_iteration_id"`
	Status                   string     `db:"status"`
	StartedAt                time.Time  `db:"started_at"`
	FinishedAt               *time.Time `db:"finished_at"`
	NumberOfCreatedDecisions int        `db:"number_of_created_decisions"`
	Manual                   bool       `db:"manual"`
}

const TABLE_SCHEDULED_EXECUTIONS = "scheduled_executions"

var ScheduledExecutionFields = []string{"id", "organization_id", "scenario_id", "scenario_iteration_id", "status", "started_at", "finished_at", "number_of_created_decisions", "manual"}

func AdaptScheduledExecution(db DBScheduledExecution, scenario models.Scenario) models.ScheduledExecution {
	return models.ScheduledExecution{
		Id:                       db.Id,
		OrganizationId:           db.OrganizationId,
		ScenarioId:               db.ScenarioId,
		ScenarioIterationId:      db.ScenarioIterationId,
		Status:                   models.ScheduledExecutionStatusFrom(db.Status),
		StartedAt:                db.StartedAt,
		FinishedAt:               db.FinishedAt,
		NumberOfCreatedDecisions: db.NumberOfCreatedDecisions,
		Scenario:                 scenario,
		Manual:                   db.Manual,
	}
}
