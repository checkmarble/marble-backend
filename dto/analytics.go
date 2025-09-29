package dto

import (
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/google/uuid"
)

type AnalyticsAvailableFiltersRequest struct {
	Start      time.Time `json:"start" validate:"required"`
	End        time.Time `json:"end" validate:"required"`
	ScenarioId uuid.UUID `json:"scenario_id" validate:"required"`
}

type AnalyticsAvailableFilter struct {
	Name   string                      `json:"name"`
	Type   models.AnalyticsType        `json:"type"`
	Source models.AnalyticsFieldSource `json:"source"`
}

func AdaptAnalyticsAvailableFilter(model models.AnalyticsFilter) AnalyticsAvailableFilter {
	source := models.AnalyticsSourceTriggerObject
	name := model.Name

	switch {
	case strings.HasPrefix(model.Name, analytics.TriggerObjectFieldPrefix):
		source = models.AnalyticsSourceTriggerObject
		name = model.Name[3:]
	case strings.HasPrefix(model.Name, analytics.DatabaseFieldPrefix):
		source = models.AnalyticsSourceIngestedData
		name = model.Name[3:]
	}

	return AnalyticsAvailableFilter{
		Name:   name,
		Type:   models.AnalyticsTypeFromColumn(model.Type),
		Source: source,
	}
}

type AnalyticsQueryFilters struct {
	Start            time.Time `json:"start" validate:"required"`
	End              time.Time `json:"end" validate:"required"`
	ScenarioId       uuid.UUID `json:"scenario_id" validate:"required"`
	ScenarioVersions []int     `json:"scenario_versions"`

	Fields []analytics.QueryObjectFilter `json:"fields"`
}

func (f AnalyticsQueryFilters) Validate() error {
	for _, tf := range f.Fields {
		if err := tf.Validate(); err != nil {
			return err
		}
	}

	return nil
}
