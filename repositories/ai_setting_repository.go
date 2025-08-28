package repositories

import (
	"context"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo MarbleDbRepository) GetAiSetting(ctx context.Context, exec Executor, orgId string) (*models.AiSetting, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.AiSettingColumns...).
		From(dbmodels.TABLE_AI_SETTING).
		Where("org_id = ?", orgId)

	return SqlToOptionalModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptAiSetting,
	)
}

func (repo MarbleDbRepository) UpsertAiSetting(ctx context.Context, exec Executor, orgId string, setting models.UpsertAiSetting) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	return ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_AI_SETTING).
			Columns(dbmodels.AiSettingColumns...).
			Values(orgId,
				setting.KYCEnrichmentSetting.Model,
				setting.KYCEnrichmentSetting.DomainFilter,
				setting.KYCEnrichmentSetting.SearchContextSize,
				setting.CaseReviewSetting.Language,
				setting.CaseReviewSetting.Structure,
				setting.CaseReviewSetting.OrgDescription,
			).
			Suffix("ON CONFLICT (org_id) DO UPDATE SET "+
				strings.Join(dbmodels.AiSettingColumns, ", ")),
	)
}
