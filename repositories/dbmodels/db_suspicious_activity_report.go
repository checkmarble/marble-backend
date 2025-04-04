package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DbSuspiciousActivityReport struct {
	Id         string     `db:"id"`
	ReportId   string     `db:"report_id"`
	CaseId     string     `db:"case_id"`
	Status     string     `db:"status"`
	Bucket     *string    `db:"bucket"`
	BlobKey    *string    `db:"blob_key"`
	CreatedBy  string     `db:"created_by"`
	UploadedBy *string    `db:"uploaded_by"`
	CreatedAt  time.Time  `db:"created_at"`
	DeletedAt  *time.Time `db:"deleted_at"`
}

const TABLE_SUSPICIOUS_ACTIVITY_REPORTS = "suspicious_activity_reports"

var SelectSuspiciousActivityReportColumns = utils.ColumnList[DbSuspiciousActivityReport]()

func AdaptSuspiciousActivityReport(db DbSuspiciousActivityReport) (models.SuspiciousActivityReport, error) {
	sar := models.SuspiciousActivityReport{
		Id:         db.Id,
		ReportId:   db.ReportId,
		CaseId:     db.CaseId,
		Status:     models.SarStatusFromString(db.Status),
		Bucket:     db.Bucket,
		BlobKey:    db.BlobKey,
		CreatedBy:  db.CreatedBy,
		UploadedBy: db.UploadedBy,
		CreatedAt:  db.CreatedAt,
		DeletedAt:  db.DeletedAt,
	}

	return sar, nil
}
