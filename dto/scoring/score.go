package scoring

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
)

type Score struct {
	Id           uuid.UUID  `json:"id"`
	RiskLevel    int        `json:"risk_level"`
	Source       string     `json:"source"`
	RulesetId    *uuid.UUID `json:"ruleset_id"`
	OverriddenBy *uuid.UUID `json:"overridden_by,omitempty"`
	Current      bool       `json:"current"`
	CreatedAt    time.Time  `json:"created_at"`
	StaleAt      *time.Time `json:"stale_at,omitempty"`

	Evaluations []*ast.NodeEvaluationDto `json:"evaluations,omitempty"`
}

type OverrideScoreRequest struct {
	RiskLevel int        `json:"risk_level" binding:"required"`
	StaleAt   *time.Time `json:"stale_at"`
}

func AdaptScore(evals []*ast.NodeEvaluationDto) func(m models.ScoringScore) Score {
	return func(m models.ScoringScore) Score {
		score := Score{
			Id:           m.Id,
			RiskLevel:    m.RiskLevel,
			Source:       string(m.Source),
			RulesetId:    m.RulesetId,
			OverriddenBy: m.OverriddenBy,
			Current:      m.DeletedAt == nil,
			CreatedAt:    m.CreatedAt,
			StaleAt: func() *time.Time {
				if m.DeletedAt != nil {
					return nil
				}
				return m.StaleAt
			}(),
		}

		if evals != nil {
			score.Evaluations = evals
		}

		return score
	}
}

type ScoreDistribution struct {
	RiskLevel int `json:"risk_level"`
	Count     int `json:"count"`
}

func AdaptScoreDistribution(m models.ScoreDistribution) ScoreDistribution {
	return ScoreDistribution{
		RiskLevel: m.RiskLevel,
		Count:     m.Count,
	}
}

type DryRun struct {
	Id           uuid.UUID           `json:"id"`
	RulesetId    uuid.UUID           `json:"ruleset_id"`
	Status       string              `json:"status"`
	RecordCount  int                 `json:"record_count"`
	Progress     float64             `json:"progress"`
	Distribution []ScoreDistribution `json:"distribution"`
	CreatedAt    time.Time           `json:"created_at"`
}

func AdaptDryRun(m models.ScoringDryRun) DryRun {
	dryRun := DryRun{
		Id:           m.Id,
		RulesetId:    m.RulesetId,
		Status:       string(m.Status),
		RecordCount:  m.RecordCount,
		Distribution: make([]ScoreDistribution, 0),
		CreatedAt:    m.CreatedAt,
	}

	processedRecords := 0

	if m.Results != nil {
		dryRun.Distribution = make([]ScoreDistribution, 0, len(m.Results))

		for score, count := range m.Results {
			processedRecords += count

			dryRun.Distribution = append(dryRun.Distribution, ScoreDistribution{
				RiskLevel: score,
				Count:     count,
			})
		}
	}

	if m.RecordCount > 0 {
		dryRun.Progress = float64(processedRecords) / float64(m.RecordCount)
	}

	return dryRun
}
