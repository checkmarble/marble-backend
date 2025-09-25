package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
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
	CreatedAt         time.Time            `db:"created_at"`
	UpdatedAt         time.Time            `db:"updated_at"`
}

const AnalyticsSettingsTable = "analytics_settings"

var AnalyticsSettingsColumns = utils.ColumnList[DbAnalyticsSettings]()

type DbAnalyticsDbField struct {
	Path []string        `json:"path"`
	Name string          `json:"name"`
	Type models.DataType `json:"type"`
}

func AdaptAnalyticsSettings(db DbAnalyticsSettings) (analytics.Settings, error) {
	return analytics.Settings{
		Id:                db.Id,
		TriggerObjectType: db.TriggerObjectType,
		TriggerFields:     db.TriggerFields,
		DbFields: pure_utils.Map(db.DbFields, func(f DbAnalyticsDbField) analytics.SettingsDbField {
			return analytics.SettingsDbField{
				Path: f.Path,
				Name: f.Name,
			}
		}),
	}, nil
}
