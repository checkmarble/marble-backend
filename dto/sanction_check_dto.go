package dto

import (
	"encoding/json"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type SanctionCheckDto struct {
	Id       string                    `json:"id"`
	Partial  bool                      `json:"partial"`
	Datasets []string                  `json:"datasets"`
	Count    int                       `json:"count"`
	Request  models.OpenSanctionsQuery `json:"request"`
	Matches  []SanctionCheckMatchDto   `json:"matches"`
}

func AdaptSanctionCheckDto(m models.SanctionCheck) SanctionCheckDto {
	sanctionCheck := SanctionCheckDto{
		Id:       m.Id,
		Partial:  m.Partial,
		Count:    m.Count,
		Datasets: make([]string, 0),
		Request:  m.Query,
		Matches:  make([]SanctionCheckMatchDto, 0),
	}

	if len(m.Query.OrgConfig.Datasets) > 0 {
		sanctionCheck.Datasets = m.Query.OrgConfig.Datasets
	}
	if len(m.Matches) > 0 {
		sanctionCheck.Matches = pure_utils.Map(m.Matches, AdaptSanctionCheckMatchDto)
	}

	return sanctionCheck
}

type SanctionCheckMatchDto struct {
	Id       string          `json:"id"`
	EntityId string          `json:"entity_id"`
	QueryIds []string        `json:"query_ids"`
	Datasets []string        `json:"datasets"`
	Payload  json.RawMessage `json:"payload"`
}

func AdaptSanctionCheckMatchDto(m models.SanctionCheckMatch) SanctionCheckMatchDto {
	match := SanctionCheckMatchDto{
		Id:       m.Id,
		EntityId: m.EntityId,
		QueryIds: m.QueryIds,
		Datasets: make([]string, 0),
		Payload:  m.Payload,
	}

	return match
}
