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

	RecordType   string     `db:"record_type"`
	RecordId     string     `db:"record_id"`
	RiskLevel    int        `db:"risk_level"`
	Source       string     `db:"source"`
	RulesetId    *uuid.UUID `db:"ruleset_id"`
	OverriddenBy *uuid.UUID `db:"overridden_by"`

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
		Id:           db.Id,
		OrgId:        db.OrgId,
		RecordType:   db.RecordType,
		RecordId:     db.RecordId,
		RiskLevel:    db.RiskLevel,
		Source:       src,
		RulesetId:    db.RulesetId,
		OverriddenBy: db.OverriddenBy,
		CreatedAt:    db.CreatedAt,
		StaleAt:      db.StaleAt,
		DeletedAt:    db.DeletedAt,
	}, nil
}

type DbScoringScoreDistribution struct {
	RiskLevel int `db:"risk_level"`
	Count     int `db:"n"` //nolint:tagliatelle
}

func AdaptScoringScoreDistribution(db DbScoringScoreDistribution) (models.ScoreDistribution, error) {
	return models.ScoreDistribution{
		RiskLevel: db.RiskLevel,
		Count:     db.Count,
	}, nil
}
