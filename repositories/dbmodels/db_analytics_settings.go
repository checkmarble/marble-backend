package dbmodels

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DbAnalyticsSettings struct {
	Id                uuid.UUID            `db:"id"`
	OrgId             uuid.UUID            `db:"org_id"`
	TriggerObjectType string               `db:"trigger_object_type"`
	TriggerFields     []string             `db:"trigger_fields"`
	DbFields          []DbAnalyticsDbField `db:"db_fields"`
}

const AnalyticsSettingsTable = "analytics_settings"

var AnalyticsSettingsColumns = utils.ColumnList[DbAnalyticsSettings]()

type DbAnalyticsDbField struct {
	Path []string        `json:"path"`
	Name string          `json:"name"`
	Type models.DataType `json:"type"`
}

func AdaptAnalyticsSettings(db DbAnalyticsSettings) (models.AnalyticsSettings, error) {
	return models.AnalyticsSettings{
		Id:                db.Id,
		TriggerObjectType: db.TriggerObjectType,
		TriggerFields:     db.TriggerFields,
		DbFields: pure_utils.Map(db.DbFields, func(f DbAnalyticsDbField) models.AnalyticsSettingsDbField {
			return models.AnalyticsSettingsDbField{
				Path: f.Path,
				Name: f.Name,
			}
		}),
	}, nil
}
