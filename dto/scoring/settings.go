package scoring

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type Settings struct {
	MaxRiskLevel int       `json:"max_risk_level"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UpdateSettingsRequest struct {
	MaxRiskLevel int `json:"max_risk_level" binding:"required"`
}

func AdaptSettings(m models.ScoringSettings) Settings {
	return Settings{
		MaxRiskLevel: m.MaxRiskLevel,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
