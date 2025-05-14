package models

import "time"

type OffloadingWatermark struct {
	OrgId         string
	TableName     string
	WatermarkTime time.Time
	WatermarkId   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
