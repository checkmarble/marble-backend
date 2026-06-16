package httpmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type HTTPOpenSanctionsDeltaFileEntity struct {
	Id         string              `json:"id"`
	Caption    string              `json:"caption"`
	Schema     string              `json:"schema"`
	Referents  []string            `json:"referents"`
	Datasets   []string            `json:"datasets"`
	Properties map[string][]string `json:"properties"`
	LastChange string              `json:"last_change"` // Cannot parse as time.Time because of missing TZ
}

func AdaptOpenSanctionDeltaFileEntityToModel(entity HTTPOpenSanctionsDeltaFileEntity) models.OpenSanctionsDeltaFileEntity {
	var lastChange *time.Time

	if t, err := time.ParseInLocation("2006-01-02T15:04:05", entity.LastChange, time.UTC); err == nil {
		lastChange = &t
	}

	return models.OpenSanctionsDeltaFileEntity{
		Id:         entity.Id,
		Caption:    entity.Caption,
		Schema:     entity.Schema,
		Referents:  entity.Referents,
		Datasets:   entity.Datasets,
		Properties: entity.Properties,
		LastChange: lastChange,
	}
}

type HTTPOpenSanctionsDeltaFileRecord struct {
	Op     string                           `json:"op"`
	Entity HTTPOpenSanctionsDeltaFileEntity `json:"entity"`
}

func AdaptOpenSanctionDeltaFileRecordToModel(record HTTPOpenSanctionsDeltaFileRecord) models.OpenSanctionsDeltaFileRecord {
	return models.OpenSanctionsDeltaFileRecord{
		Op:     models.OpenSanctionsDeltaFileRecordOpFromString(record.Op),
		Entity: AdaptOpenSanctionDeltaFileEntityToModel(record.Entity),
	}
}
