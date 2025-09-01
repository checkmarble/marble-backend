package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
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
	error := ""
	if log.Error != nil {
		error = *log.Error
	}

	return UploadLogDto{
		Status:          string(log.UploadStatus),
		StartedAt:       log.StartedAt,
		FinishedAt:      log.FinishedAt,
		LinesProcessed:  log.LinesProcessed,
		NumRowsIngested: log.RowsIngested,
		Error:           error,
	}
}
