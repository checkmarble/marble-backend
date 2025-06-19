package dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type Screening struct {
	Id           string                           `json:"id"`
	Status       string                           `json:"status"`
	Query        json.RawMessage                  `json:"query"`
	InitialQuery []models.OpenSanctionsCheckQuery `json:"initial_query"`
	Partial      bool                             `json:"partial"`

	MatchCount int              `json:"match_count"`
	Matches    []ScreeningMatch `json:"matches,omitzero"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ScreeningMatch struct {
	Id      string          `json:"id"`
	Queries []string        `json:"queries"`
	Status  string          `json:"status"`
	Payload json.RawMessage `json:"payload"`
}

func AdaptScreening(includeMatches bool) func(models.ScreeningWithMatches) Screening {
	return func(model models.ScreeningWithMatches) Screening {
		sc := Screening{
			Id:           model.Id,
			Status:       model.Status.String(),
			Query:        model.SearchInput,
			InitialQuery: model.InitialQuery,
			Partial:      model.Partial,
			MatchCount:   len(model.Matches),
			CreatedAt:    model.CreatedAt,
			UpdatedAt:    model.UpdatedAt,
		}

		if includeMatches {
			sc.Matches = []ScreeningMatch{}

			if model.Matches != nil {
				sc.Matches = pure_utils.Map(model.Matches, AdaptScreeningMatch)
			}
		}

		return sc
	}
}

func AdaptScreeningMatch(model models.ScreeningMatch) ScreeningMatch {
	return ScreeningMatch{
		Id:      model.Id,
		Queries: model.QueryIds,
		Status:  model.Status.String(),
		Payload: model.Payload,
	}
}

type ScreeningWhitelist struct {
	Counterparty string    `json:"counterparty"`
	EntityId     string    `json:"entity_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func AdaptScreeningWhitelist(model models.ScreeningWhitelist) ScreeningWhitelist {
	return ScreeningWhitelist{
		Counterparty: model.CounterpartyId,
		EntityId:     model.EntityId,
		CreatedAt:    model.CreatedAt,
	}
}
