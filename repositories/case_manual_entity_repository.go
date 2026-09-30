package repositories

import (
	"context"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) ListCaseManualEntities(ctx context.Context, exec Executor, orgId uuid.UUID, caseId string) ([]models.CaseManualEntity, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().Select(dbmodels.CaseManualEntityColumns...).From("case_manual_entities").Where(squirrel.Eq{"org_id": orgId, "case_id": caseId})
	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCaseManualEntity)
}
func (repo *MarbleDbRepository) InsertCaseManualEntity(ctx context.Context, exec Executor, orgId uuid.UUID, caseId string, ref models.CaseEntityRef) (*models.CaseManualEntity, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().Insert("case_manual_entities").
		Columns(dbmodels.CaseManualEntityColumns...).
		Values(pure_utils.NewId(), orgId, caseId, ref.TableName, ref.ObjectId).
		Suffix("ON CONFLICT (case_id,table_name,object_id) DO NOTHING RETURNING " + strings.Join(dbmodels.CaseManualEntityColumns, ","))
	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptCaseManualEntity)
}
func (repo *MarbleDbRepository) DeleteCaseManualEntity(ctx context.Context, exec Executor, orgId uuid.UUID, caseId string, ref models.CaseEntityRef) (*models.CaseManualEntity, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	query := NewQueryBuilder().Delete("case_manual_entities").Where(squirrel.Eq{"org_id": orgId, "case_id": caseId, "table_name": ref.TableName, "object_id": ref.ObjectId}).Suffix("RETURNING " + strings.Join(dbmodels.CaseManualEntityColumns, ","))
	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptCaseManualEntity)
}
