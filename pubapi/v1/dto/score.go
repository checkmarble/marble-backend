package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type RiskLevel struct {
	Source       string     `json:"source"`
	RiskLevel    int        `json:"risk_level"`
	Current      bool       `json:"current"`
	OverriddenBy *Ref       `json:"overridden_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	StaleAt      *time.Time `json:"stale_at,omitempty"`
}

func AdaptRiskLevel(rl models.ScoringScore, overriddenBy *Ref) RiskLevel {
	return RiskLevel{
		Source:       string(rl.Source),
		RiskLevel:    rl.RiskLevel,
		Current:      rl.DeletedAt == nil,
		CreatedAt:    rl.CreatedAt,
		OverriddenBy: overriddenBy,
		StaleAt: func() *time.Time {
			if rl.DeletedAt != nil {
				return nil
			}
			return rl.StaleAt
		}(),
	}
}

type RiskLevelOverride struct {
	RiskLevel int `json:"risk_level"`
}
