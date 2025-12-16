package httpmodels

import "github.com/checkmarble/marble-backend/models"

type HTTPOpenSanctionsDeltaFileEntity struct {
	Id         string              `json:"id"`
	Caption    string              `json:"caption"`
	Schema     string              `json:"schema"`
	Referents  []string            `json:"referents"`
	Datasets   []string            `json:"datasets"`
	FirstSeen  string              `json:"first_seen"`
	LastSeen   string              `json:"last_seen"`
	LastChange string              `json:"last_change"`
	Properties map[string][]string `json:"properties"`
	Target     bool                `json:"target"`
}

func AdaptOpenSanctionDeltaFileEntityToModel(entity HTTPOpenSanctionsDeltaFileEntity) models.OpenSanctionsDeltaFileEntity {
	return models.OpenSanctionsDeltaFileEntity{
		Id:         entity.Id,
		Caption:    entity.Caption,
		Schema:     entity.Schema,
		Referents:  entity.Referents,
		Datasets:   entity.Datasets,
		FirstSeen:  entity.FirstSeen,
		LastSeen:   entity.LastSeen,
		LastChange: entity.LastChange,
		Properties: entity.Properties,
		Target:     entity.Target,
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
