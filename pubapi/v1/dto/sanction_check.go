package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
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
		Matches:   make([]SanctionCheckMatch, len(model.Matches)),
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}

	for idx, match := range model.Matches {
		sc.Matches[idx] = SanctionCheckMatch{
			Id:      match.Id,
			Queries: match.QueryIds,
			Status:  match.Status.String(),
			Payload: match.Payload,
		}
	}

	return sc
}
