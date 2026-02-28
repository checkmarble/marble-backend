package scoring

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type Score struct {
	Id          uuid.UUID  `json:"id"`
	Score       int        `json:"score"`
	Source      string     `json:"source"`
	OverridenBy *uuid.UUID `json:"overriden_by,omitempty"`
	Current     bool       `json:"current"`
	CreatedAt   time.Time  `json:"created_at"`
	StaleAt     *time.Time `json:"stale_at,omitempty"`
}

type OverrideScoreRequest struct {
	Score   int        `json:"score"`
	StaleAt *time.Time `json:"stale_at"`
}

func AdaptScore(m models.ScoringScore) Score {
	return Score{
		Id:          m.Id,
		Score:       m.Score,
		Source:      string(m.Source),
		OverridenBy: m.OverridenBy,
		Current:     m.DeletedAt == nil,
		CreatedAt:   m.CreatedAt,
		StaleAt: func() *time.Time {
			if m.DeletedAt != nil {
				return nil
			}
			return m.StaleAt
		}(),
	}
}
