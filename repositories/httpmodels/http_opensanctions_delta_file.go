package httpmodels

import "github.com/checkmarble/marble-backend/models"

type HTTPOpenSanctionsDeltaFileEntity struct {
	Id         string              `json:"id"`
	Caption    string              `json:"caption"`
	Schema     string              `json:"schema"`
	Referents  []string            `json:"referents"`
	Datasets   []string            `json:"datasets"`
	Properties map[string][]string `json:"properties"`
}

func AdaptOpenSanctionDeltaFileEntityToModel(entity HTTPOpenSanctionsDeltaFileEntity) models.OpenSanctionsDeltaFileEntity {
	return models.OpenSanctionsDeltaFileEntity{
		Id:         entity.Id,
		Caption:    entity.Caption,
		Schema:     entity.Schema,
		Referents:  entity.Referents,
		Datasets:   entity.Datasets,
		Properties: entity.Properties,
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
