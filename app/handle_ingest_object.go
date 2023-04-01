package app

import (
	"marble/marble-backend/app/data_model"
	"marble/marble-backend/app/dynamic_reading"
)

func (a *App) IngestObject(dynamicStructWithReader dynamic_reading.DynamicStructWithReader, table data_model.Table) (err error) {
	return a.repository.IngestObject(dynamicStructWithReader, table)
}
