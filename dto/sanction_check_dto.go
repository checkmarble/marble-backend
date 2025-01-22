package dto

import (
	"encoding/json"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/cockroachdb/errors"
)

var (
	ValidSanctionCheckStatuses      = []string{"in_review", "confirmed_hit", "error"}
	ValidSanctionCheckMatchStatuses = []string{"pending", "confirmed_hit", "no_hit"}
)

type SanctionCheckDto struct {
	Id          string                    `json:"id"`
	Datasets    []string                  `json:"datasets"`
	Request     models.OpenSanctionsQuery `json:"request"`
	Status      string                    `json:"status"`
	Partial     bool                      `json:"partial"`
	Count       int                       `json:"count"`
	IsManual    bool                      `json:"is_manual"`
	RequestedBy *string                   `json:"requested_by,omitempty"`
	Matches     []SanctionCheckMatchDto   `json:"matches"`
}

func AdaptSanctionCheckDto(m models.SanctionCheck) SanctionCheckDto {
	sanctionCheck := SanctionCheckDto{
		Id:          m.Id,
		Datasets:    make([]string, 0),
		Request:     m.Query,
		Status:      m.Status,
		Partial:     m.Partial,
		Count:       m.Count,
		IsManual:    m.IsManual,
		RequestedBy: m.RequestedBy,
		Matches:     make([]SanctionCheckMatchDto, 0),
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
	Status   string          `json:"status"`
	Datasets []string        `json:"datasets"`
	Payload  json.RawMessage `json:"payload"`
}

func AdaptSanctionCheckMatchDto(m models.SanctionCheckMatch) SanctionCheckMatchDto {
	match := SanctionCheckMatchDto{
		Id:       m.Id,
		EntityId: m.EntityId,
		Status:   m.Status,
		QueryIds: m.QueryIds,
		Datasets: make([]string, 0),
		Payload:  m.Payload,
	}

	return match
}

type SanctionCheckMatchUpdateDto struct {
	Status string `json:"status"`
}

func (dto SanctionCheckMatchUpdateDto) Validate() error {
	if !slices.Contains(ValidSanctionCheckMatchStatuses, dto.Status) {
		return errors.Wrap(models.BadParameterError,
			"invalid status for sanction check match")
	}

	return nil
}
