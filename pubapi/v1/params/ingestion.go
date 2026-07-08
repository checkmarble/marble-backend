package params

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pubapi/types"
)

type IngestionParams struct {
	SkipInitialScreening bool     `form:"skip_screening"`
	MonitorObjects       bool     `form:"monitor"`
	ContinuousConfigIds  []string `form:"monitoring_config_id" binding:"required_if=MonitorObjects true,omitempty,dive,uuid"`
}

type UploadLogParams struct {
	types.PaginationParams

	Status *models.UploadStatus `form:"status" binding:"oneof=pending processing success failure"`
}

func (p UploadLogParams) ToModel() models.UploadLogFilters {
	return models.UploadLogFilters{
		Status: p.Status,
	}
}
