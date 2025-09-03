package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type AnalyticsQueryFilters struct {
	Start            time.Time `json:"start" validate:"required"`
	End              time.Time `json:"end" validate:"required"`
	ScenarioId       uuid.UUID `json:"scenario_id" validate:"required"`
	ScenarioVersions []int     `json:"scenario_versions"`

	Trigger []models.AnalyticsQueryObjectFilter `json:"trigger"`
}

func (f AnalyticsQueryFilters) Validate() error {
	for _, tf := range f.Trigger {
		if err := tf.Validate(); err != nil {
			return err
		}
	}

	return nil
}
