package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type DbScoringScore struct {
	Id    uuid.UUID `db:"id"`
	OrgId uuid.UUID `db:"org_id"`

	EntityType  string     `db:"entity_type"`
	EntityId    string     `db:"entity_id"`
	Score       int        `db:"score"`
	Source      string     `db:"source"`
	OverridenBy *uuid.UUID `db:"overriden_by"`

	CreatedAt time.Time  `db:"created_at"`
	StaleAt   *time.Time `db:"stale_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

const TABLE_SCORING_SCORES = "scoring_scores"

var SelectScoringScoresColumns = utils.ColumnList[DbScoringScore]()

func AdaptScoringScore(db DbScoringScore) (models.ScoringScore, error) {
	src := models.ScoreSourceFrom(db.Source)
	if src == models.ScoreSourceUnknown {
		return models.ScoringScore{}, errors.Newf("unknown source type %s", db.Source)
	}

	return models.ScoringScore{
		Id:          db.Id,
		OrgId:       db.OrgId,
		EntityType:  db.EntityType,
		EntityId:    db.EntityId,
		Score:       db.Score,
		Source:      src,
		OverridenBy: db.OverridenBy,
		CreatedAt:   db.CreatedAt,
		StaleAt:     db.StaleAt,
		DeletedAt:   db.DeletedAt,
	}, nil
}
