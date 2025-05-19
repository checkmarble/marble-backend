package dto

import (
	"mime/multipart"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type SuspiciousActivityReportDto struct {
	ReportId   string    `json:"id"` //nolint:tagliatelle
	Status     string    `json:"status"`
	HasFile    bool      `json:"has_file"`
	CreatedBy  string    `json:"created_by"`
	UploadedBy *string   `json:"uploaded_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type SuspiciousActivityReportParams struct {
	Status *string               `form:"status" binding:"omitempty,oneof=pending completed"`
	File   *multipart.FileHeader `form:"file"`
}

func AdaptSuspiciousActivityReportDto(model models.SuspiciousActivityReport) SuspiciousActivityReportDto {
	return SuspiciousActivityReportDto{
		ReportId:   model.ReportId,
		Status:     model.Status.String(),
		HasFile:    model.UploadedBy != nil,
		CreatedBy:  model.CreatedBy,
		UploadedBy: model.UploadedBy,
		CreatedAt:  model.CreatedAt,
	}
}
