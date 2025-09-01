package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

func (repo MarbleDbRepository) GetAiSettingById(ctx context.Context, exec Executor, id uuid.UUID) (*models.AiSetting, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.AiSettingColumns...).
		From(dbmodels.TABLE_AI_SETTING).
		Where("id = ?", id)

	return SqlToOptionalModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptAiSetting,
	)
}

func (repo MarbleDbRepository) UpsertAiSetting(
	ctx context.Context,
	exec Executor,
	aiSettingId *uuid.UUID,
	setting models.UpsertAiSetting,
) (models.AiSetting, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.AiSetting{}, err
	}

	if aiSettingId == nil {
		aiSettingId = utils.Ptr(uuid.New())
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_AI_SETTING).
		Columns(dbmodels.AiSettingColumnsInsert...).
		Values(
			aiSettingId,
			setting.KYCEnrichmentSetting.Model,
			setting.KYCEnrichmentSetting.DomainFilter,
			setting.KYCEnrichmentSetting.SearchContextSize,
			setting.CaseReviewSetting.Language,
			setting.CaseReviewSetting.Structure,
			setting.CaseReviewSetting.OrgDescription,
		).
		Suffix("ON CONFLICT (id) DO UPDATE SET " +
			"kyc_enrichment_model = EXCLUDED.kyc_enrichment_model, " +
			"kyc_enrichment_domain_filter = EXCLUDED.kyc_enrichment_domain_filter, " +
			"kyc_enrichment_search_context_size = EXCLUDED.kyc_enrichment_search_context_size, " +
			"case_review_language = EXCLUDED.case_review_language, " +
			"case_review_structure = EXCLUDED.case_review_structure, " +
			"case_review_org_description = EXCLUDED.case_review_org_description, " +
			"updated_at = CURRENT_TIMESTAMP",
		).
		Suffix("RETURNING *")

	return SqlToModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptAiSetting,
	)
}
