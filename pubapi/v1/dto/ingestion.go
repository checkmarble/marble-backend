package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/google/uuid"
)

type AsyncUpload struct {
	UploadUrl string `json:"upload_url"`
}

type UploadLog struct {
	Id            uuid.UUID  `json:"id"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	RowsProcessed int        `json:"rows_processed"`
	RowsIngested  int        `json:"rows_ingested"`
	Error         string     `json:"error,omitempty"`
}

func AdaptUploadLog(log models.UploadLog) UploadLog {
	return UploadLog{
		Id:            log.Id,
		Status:        string(log.UploadStatus),
		StartedAt:     log.StartedAt,
		FinishedAt:    log.FinishedAt,
		RowsProcessed: max(log.LinesProcessed, log.RowsIngested),
		RowsIngested:  log.RowsIngested,
		Error:         pure_utils.PtrValueOrDefault(log.InputError, ""),
	}
}
