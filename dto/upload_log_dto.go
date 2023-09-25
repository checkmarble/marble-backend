package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type APIUploadLog struct {
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	LinesProcessed int        `json:"lines_processed"`
}

func AdaptUploadLogDto(log models.UploadLog) APIUploadLog {
	return APIUploadLog{
		Status:         string(log.UploadStatus),
		StartedAt:      log.StartedAt,
		FinishedAt:     log.FinishedAt,
		LinesProcessed: log.LinesProcessed,
	}
}
