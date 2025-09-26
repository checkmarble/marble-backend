package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) GetAnalyticsSettings(ctx context.Context, exec Executor, orgId string) (map[string]analytics.Settings, error) {
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
