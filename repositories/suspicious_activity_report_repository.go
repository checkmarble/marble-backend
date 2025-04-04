package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) ListSuspiciousActivityReportsByCaseId(ctx context.Context, exec Executor,
	caseId string,
) ([]models.SuspiciousActivityReport, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSuspiciousActivityReportColumns...).
		From(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS).
		Where(squirrel.Eq{
			"case_id":    caseId,
			"deleted_at": nil,
		})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSuspiciousActivityReport)
}

func (repo *MarbleDbRepository) GetSuspiciousActivityReportById(ctx context.Context,
	exec Executor,
	caseId, id string,
	forUpdate bool,
) (models.SuspiciousActivityReport, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SuspiciousActivityReport{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSuspiciousActivityReportColumns...).
		From(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS).
		Where(squirrel.Eq{
			"case_id":    caseId,
			"report_id":  id,
			"deleted_at": nil,
		})

	if forUpdate {
		sql = sql.Suffix("for update")
	}

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSuspiciousActivityReport)
}

func (repo *MarbleDbRepository) CreateSuspiciousActivityReport(ctx context.Context,
	exec Executor,
	req models.CreateSuspiciousActivityReportRequest,
) (models.SuspiciousActivityReport, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SuspiciousActivityReport{}, err
	}

	reportId := req.ReportId
	if reportId == nil {
		reportId = utils.Ptr(uuid.NewString())
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS).
		Columns("report_id", "case_id", "status", "bucket", "blob_key", "created_by", "uploaded_by").
		Values(
			reportId,
			req.CaseId,
			req.Status.String(),
			req.Bucket,
			req.BlobKey,
			req.CreatedBy,
			req.UploadedBy,
		).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSuspiciousActivityReport)
}

func (repo *MarbleDbRepository) UpdateSuspiciousActivityReport(ctx context.Context,
	exec Executor,
	req models.UpdateSuspiciousActivityReportRequest,
) (models.SuspiciousActivityReport, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SuspiciousActivityReport{}, err
	}

	values := map[string]any{
		"status": req.Status,
	}

	if req.DeletedAt != nil {
		values["deleted_at"] = utils.Ptr(time.Now())
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS).
		SetMap(values).
		Where(squirrel.Eq{
			"case_id":    req.CaseId,
			"report_id":  req.ReportId,
			"deleted_at": nil,
		}).
		Suffix("returning *")

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSuspiciousActivityReport)
}

func (repo *MarbleDbRepository) UploadSuspiciousActivityReport(ctx context.Context, tx Transaction,
	sar models.SuspiciousActivityReport,
	req models.UploadSuspiciousActivityReportRequest,
) (models.SuspiciousActivityReport, error) {
	if sar.Bucket == nil || sar.BlobKey == nil {
		sql := NewQueryBuilder().
			Update(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS).
			Set("bucket", req.Bucket).
			Set("blob_key", req.BlobKey).
			Where(squirrel.Eq{
				"case_id":    req.CaseId,
				"report_id":  req.ReportId,
				"deleted_at": nil,
			}).
			Suffix("returning *")

		return SqlToModel(ctx, tx, sql, dbmodels.AdaptSuspiciousActivityReport)
	}

	_, err := repo.UpdateSuspiciousActivityReport(ctx, tx, models.UpdateSuspiciousActivityReportRequest{
		CaseId:    req.CaseId,
		ReportId:  req.ReportId,
		DeletedAt: utils.Ptr(time.Now()),
	})
	if err != nil {
		return models.SuspiciousActivityReport{}, err
	}

	create := models.CreateSuspiciousActivityReportRequest{
		CaseId:     sar.CaseId,
		ReportId:   &sar.ReportId,
		Status:     sar.Status,
		Bucket:     &req.Bucket,
		BlobKey:    &req.BlobKey,
		CreatedBy:  models.UserId(sar.CreatedBy),
		UploadedBy: &req.UploadedBy,
	}

	return repo.CreateSuspiciousActivityReport(ctx, tx, create)
}

func (repo *MarbleDbRepository) DeleteSuspiciousActivityReport(ctx context.Context,
	exec Executor,
	req models.UpdateSuspiciousActivityReportRequest,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SUSPICIOUS_ACTIVITY_REPORTS).
		Set("deleted_at", time.Now()).
		Where(squirrel.Eq{
			"case_id":    req.CaseId,
			"report_id":  req.ReportId,
			"deleted_at": nil,
		})

	return ExecBuilder(ctx, exec, sql)
}
