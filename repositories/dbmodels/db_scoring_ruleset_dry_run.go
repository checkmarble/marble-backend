package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbScoringDryRun struct {
	Id          uuid.UUID           `db:"id"`
	RulesetId   uuid.UUID           `db:"ruleset_id"`
	Status      models.DryRunStatus `db:"status"`
	RecordCount int                 `db:"record_count"`
	Results     map[int]int         `db:"results"`
	CreatedAt   time.Time           `db:"created_at"`
}

const (
	TABLE_SCORING_DRY_RUNS = "scoring_dry_runs"
)

var SelectScoringDryRunsColumns = utils.ColumnList[DbScoringDryRun]()

func AdaptScoringDryRun(db DbScoringDryRun) (models.ScoringDryRun, error) {
	return models.ScoringDryRun{
		Id:          db.Id,
		RulesetId:   db.RulesetId,
		Status:      db.Status,
		RecordCount: db.RecordCount,
		Results:     db.Results,
		CreatedAt:   db.CreatedAt,
	}, nil
}
