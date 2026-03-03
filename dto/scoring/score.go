package scoring

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
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
}

type OverrideScoreRequest struct {
	RiskLevel int        `json:"risk_level"`
	StaleAt   *time.Time `json:"stale_at"`
}

func AdaptScore(m models.ScoringScore) Score {
	return Score{
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
}
