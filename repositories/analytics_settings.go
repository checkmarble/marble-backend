package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetAnalyticsSettings(ctx context.Context, exec Executor, orgId uuid.UUID) (map[string]analytics.Settings, error) {
	query := NewQueryBuilder().
		Select(dbmodels.AnalyticsSettingsColumns...).
		From(dbmodels.AnalyticsSettingsTable).
		Where("org_id = ?", orgId)

	rows, err := SqlToListOfModels(ctx, exec, query, dbmodels.AdaptAnalyticsSettings)
	if err != nil {
		return nil, err
	}

	settings := make(map[string]analytics.Settings, len(rows))

	for _, s := range rows {
		settings[s.TriggerObjectType] = s
	}

	return settings, nil
}

func (repo *MarbleDbRepository) UpdateAnalyticsSettings(ctx context.Context, exec Executor,
	orgId uuid.UUID, triggerObjectType string, newSettings dto.AnalyticsSettingDto,
) (analytics.Settings, error) {
	query := NewQueryBuilder().
		Insert(dbmodels.AnalyticsSettingsTable).
		Columns("org_id", "trigger_object_type", "trigger_fields", "db_fields").
		Values(orgId, triggerObjectType, newSettings.TriggerObjectFields, newSettings.IngestedDataFields).
		Suffix(`
			on conflict (org_id, trigger_object_type) do update
			set
				trigger_fields = excluded.trigger_fields,
				db_fields = excluded.db_fields
		`).
		Suffix("returning *")

	settings, err := SqlToModel(ctx, exec, query, dbmodels.AdaptAnalyticsSettings)
	if err != nil {
		return analytics.Settings{}, err
	}

	return settings, nil
}
