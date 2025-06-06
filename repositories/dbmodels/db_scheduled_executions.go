package dbmodels

import (
	"strconv"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBScheduledExecution struct {
	Id                         string     `db:"id"`
	OrganizationId             string     `db:"organization_id"`
	ScenarioId                 string     `db:"scenario_id"`
	ScenarioIterationId        string     `db:"scenario_iteration_id"`
	Status                     string     `db:"status"`
	StartedAt                  time.Time  `db:"started_at"`
	FinishedAt                 *time.Time `db:"finished_at"`
	NumberOfCreatedDecisions   int        `db:"number_of_created_decisions"`
	NumberOfEvaluatedDecisions int        `db:"number_of_evaluated_decisions"`
	NumberOfPlannedDecisions   *int       `db:"number_of_planned_decisions"`
	Manual                     bool       `db:"manual"`
}

const TABLE_SCHEDULED_EXECUTIONS = "scheduled_executions"

var ScheduledExecutionFields = utils.ColumnList[DBScheduledExecution]()

func AdaptScheduledExecution(db DBScheduledExecution, scenario models.Scenario,
	iteration models.ScenarioIteration,
) models.ScheduledExecution {
	return models.ScheduledExecution{
		Id:                         db.Id,
		OrganizationId:             db.OrganizationId,
		ScenarioId:                 db.ScenarioId,
		ScenarioIterationId:        db.ScenarioIterationId,
		ScenarioVersion:            strconv.Itoa(utils.Or(iteration.Version, 0)),
		Status:                     models.ScheduledExecutionStatusFrom(db.Status),
		StartedAt:                  db.StartedAt,
		FinishedAt:                 db.FinishedAt,
		NumberOfCreatedDecisions:   db.NumberOfCreatedDecisions,
		NumberOfEvaluatedDecisions: db.NumberOfEvaluatedDecisions,
		NumberOfPlannedDecisions:   db.NumberOfPlannedDecisions,
		Scenario:                   scenario,
		Manual:                     db.Manual,
	}
}
