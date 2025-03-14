package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type SanctionCheck struct {
	Id      string          `json:"id"`
	Status  string          `json:"status"`
	Query   json.RawMessage `json:"query"`
	Partial bool            `json:"partial"`

	Matches []SanctionCheckMatch `json:"matches"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SanctionCheckMatch struct {
	Id      string          `json:"id"`
	Queries []string        `json:"queries"`
	Status  string          `json:"status"`
	Payload json.RawMessage `json:"payload"`
}

func AdaptSanctionCheck(model models.SanctionCheckWithMatches) SanctionCheck {
	sc := SanctionCheck{
		Id:        model.Id,
		Status:    model.Status.String(),
		Query:     model.SearchInput,
		Partial:   model.Partial,
		Matches:   pure_utils.Map(model.Matches, AdaptSanctionCheckMatch),
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}

	return sc
}

func AdaptSanctionCheckMatch(model models.SanctionCheckMatch) SanctionCheckMatch {
	return SanctionCheckMatch{
		Id:      model.Id,
		Queries: model.QueryIds,
		Status:  model.Status.String(),
		Payload: model.Payload,
	}
}

type SanctionCheckWhitelist struct {
	Counterparty string    `json:"counterparty"`
	EntityId     string    `json:"entity_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func AdaptSanctionCheckWhitelist(model models.SanctionCheckWhitelist) SanctionCheckWhitelist {
	return SanctionCheckWhitelist{
		Counterparty: model.CounterpartyId,
		EntityId:     model.EntityId,
		CreatedAt:    model.CreatedAt,
	}
}
