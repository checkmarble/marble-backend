package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const caseManualEntityColumns = "id, org_id, case_id, table_name, object_id"

func (repo *MarbleDbRepository) ListCaseManualEntities(ctx context.Context, exec Executor, orgId uuid.UUID, caseId string) ([]models.CaseManualEntity, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	rows, err := exec.Query(ctx, "SELECT "+caseManualEntityColumns+" FROM case_manual_entities WHERE org_id=$1 AND case_id=$2 ORDER BY table_name, object_id", orgId, caseId)
	if err != nil {
		return nil, err
	}
	dbs, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbmodels.DBCaseManualEntity])
	if err != nil {
		return nil, err
	}
	result := make([]models.CaseManualEntity, len(dbs))
	for i, db := range dbs {
		result[i] = dbmodels.AdaptCaseManualEntity(db)
	}
	return result, nil
}

func (repo *MarbleDbRepository) InsertCaseManualEntity(ctx context.Context, exec Executor, orgId uuid.UUID, caseId string, ref models.CaseEntityRef) (*models.CaseManualEntity, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	row := exec.QueryRow(ctx, `INSERT INTO case_manual_entities (id,org_id,case_id,table_name,object_id)
		SELECT $1,c.org_id,c.id,$4,$5 FROM cases c WHERE c.id=$2 AND c.org_id=$3
		ON CONFLICT (case_id,table_name,object_id) DO NOTHING
		RETURNING `+caseManualEntityColumns, uuid.New(), caseId, orgId, ref.TableName, ref.ObjectId)
	var db dbmodels.DBCaseManualEntity
	err := row.Scan(&db.Id, &db.OrgId, &db.CaseId, &db.TableName, &db.ObjectId)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	model := dbmodels.AdaptCaseManualEntity(db)
	return &model, nil
}

func (repo *MarbleDbRepository) DeleteCaseManualEntity(ctx context.Context, exec Executor, orgId uuid.UUID, caseId string, ref models.CaseEntityRef) (*models.CaseManualEntity, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	row := exec.QueryRow(ctx, `DELETE FROM case_manual_entities WHERE org_id=$1 AND case_id=$2 AND table_name=$3 AND object_id=$4 RETURNING `+caseManualEntityColumns, orgId, caseId, ref.TableName, ref.ObjectId)
	var db dbmodels.DBCaseManualEntity
	err := row.Scan(&db.Id, &db.OrgId, &db.CaseId, &db.TableName, &db.ObjectId)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	model := dbmodels.AdaptCaseManualEntity(db)
	return &model, nil
}
