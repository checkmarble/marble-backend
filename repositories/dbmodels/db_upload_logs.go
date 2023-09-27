package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type DBUploadLog struct {
	Id             string     `db:"id"`
	OrganizationId string     `db:"org_id"`
	UserId         string     `db:"user_id"`
	FileName       string     `db:"file_name"`
	Status         string     `db:"status"`
	StartedAt      time.Time  `db:"started_at"`
	FinishedAt     *time.Time `db:"finished_at"`
	LinesProcessed int        `db:"lines_processed"`
}

const TABLE_UPLOAD_LOGS = "upload_logs"

var SelectUploadLogColumn = utils.ColumnList[DBUploadLog]()

func AdaptUploadLog(db DBUploadLog) models.UploadLog {
	return models.UploadLog{
		Id:             db.Id,
		OrganizationId: db.OrganizationId,
		UserId:         db.UserId,
		FileName:       db.FileName,
		UploadStatus:   models.UploadStatusFrom(db.Status),
		StartedAt:      db.StartedAt,
		FinishedAt:     db.FinishedAt,
		LinesProcessed: db.LinesProcessed,
	}
}
