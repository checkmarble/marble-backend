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
	ObjectType    string     `json:"object_type"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	RowsProcessed int        `json:"rows_processed"`
	RowsIngested  int        `json:"rows_ingested"`
	Error         string     `json:"error,omitempty"`
	ErrorCode     string     `json:"error_code,omitempty"`
}

func AdaptUploadLog(log models.UploadLog) UploadLog {
	errorMessage := pure_utils.PtrValueOrDefault(log.InputError, "")
	if errorMessage == "" {
		switch log.ErrorCode {
		case models.IngestionFailureInvalidInput,
			models.IngestionFailureGlobalTimeout,
			models.IngestionFailureUploadNotReceived,
			models.IngestionFailureFileTooLarge:
			errorMessage = pure_utils.PtrValueOrDefault(log.Error, "")
		}
	}
	return UploadLog{
		Id:            log.Id,
		ObjectType:    log.TableName,
		Status:        string(log.UploadStatus),
		StartedAt:     log.StartedAt,
		FinishedAt:    log.FinishedAt,
		RowsProcessed: max(log.LinesProcessed, log.RowsIngested),
		RowsIngested:  log.RowsIngested,
		Error:         errorMessage,
		ErrorCode:     string(log.ErrorCode),
	}
}
