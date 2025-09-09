package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
)

func (repo MarbleDbRepository) GetAiSetting(ctx context.Context, exec Executor, orgId string) (*models.AiSetting, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.AiSettingColumns...).
		From(dbmodels.TABLE_AI_SETTING).
		Where("org_id = ?", orgId)

	settings, err := SqlToListOfModels(ctx, exec, query, func(db dbmodels.DBAiSetting) (dbmodels.DBAiSetting, error) {
		return db, nil
	})
	if err != nil {
		return nil, err
	}

	if len(settings) == 0 {
		return nil, nil
	}

	aiSetting, err := dbmodels.AdaptAiSetting(settings)
	if err != nil {
		return nil, err
	}

	return &aiSetting, nil
}

func (repo MarbleDbRepository) PutAiSetting(
	ctx context.Context,
	exec Executor,
	orgId string,
	setting models.UpsertAiSetting,
) (models.AiSetting, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.AiSetting{}, err
	}

	if err := repo.upsertAiSettingType(ctx, exec, orgId,
		dbmodels.AI_SETTING_TYPE_KYC_ENRICHMENT,
		setting.KYCEnrichmentSetting); err != nil {
		return models.AiSetting{}, err
	}
	if err := repo.upsertAiSettingType(ctx, exec, orgId,
		dbmodels.AI_SETTING_TYPE_CASE_REVIEW,
		setting.CaseReviewSetting); err != nil {
		return models.AiSetting{}, err
	}

	// Get the complete updated setting
	result, err := repo.GetAiSetting(ctx, exec, orgId)
	if err != nil {
		return models.AiSetting{}, err
	}
	if result == nil {
		return models.AiSetting{}, errors.New("failed to retrieve updated AI setting")
	}

	return *result, nil
}

func (repo MarbleDbRepository) upsertAiSettingType(
	ctx context.Context,
	exec Executor,
	orgId string,
	settingType string,
	value models.AiSettingEntity,
) error {
	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_AI_SETTING).
		Columns("org_id", "type", "value").
		Values(orgId, settingType, value).
		Suffix("ON CONFLICT (org_id, type) DO UPDATE SET " +
			"value = EXCLUDED.value, " +
			"updated_at = CURRENT_TIMESTAMP")

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}
	_, err = exec.Exec(ctx, sql, args...)
	return err
}
