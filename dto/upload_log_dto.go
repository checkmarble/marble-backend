package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type UploadLogDto struct {
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	LinesProcessed  int        `json:"lines_processed"`
	NumRowsIngested int        `json:"num_rows_ingested"`
	Error           string     `json:"error"`
}

func AdaptUploadLogDto(log models.UploadLog) UploadLogDto {
	return UploadLogDto{
		Status:          string(log.UploadStatus),
		StartedAt:       log.StartedAt,
		FinishedAt:      log.FinishedAt,
		LinesProcessed:  max(log.LinesProcessed, log.RowsIngested),
		NumRowsIngested: log.RowsIngested,
		Error:           utils.Or(log.InputError, ""),
	}
}
