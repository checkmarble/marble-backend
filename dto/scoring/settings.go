package scoring

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type Settings struct {
	MaxScore  int       `json:"max_score"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateSettingsRequest struct {
	MaxScore int `json:"max_score" binding:"required"`
}

func AdaptSettings(m models.ScoringSettings) Settings {
	return Settings{
		MaxScore:  m.MaxScore,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
