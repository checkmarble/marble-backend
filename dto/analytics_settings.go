package dto

import (
	"github.com/checkmarble/marble-backend/models/analytics"
)

type AnalyticsSettingDto struct {
	TriggerObjectFields []string                    `json:"trigger_object_fields"`
	IngestedDataFields  []analytics.SettingsDbField `json:"ingested_data_fields"`
}

func AdaptAnalyticsSettings(model analytics.Settings) AnalyticsSettingDto {
	if model.TriggerFields == nil {
		model.TriggerFields = make([]string, 0)
	}
	if model.DbFields == nil {
		model.DbFields = make([]analytics.SettingsDbField, 0)
	}

	return AnalyticsSettingDto{
		TriggerObjectFields: model.TriggerFields,
		IngestedDataFields:  model.DbFields,
	}
}
