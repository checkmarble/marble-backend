package httpmodels

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"

	"github.com/checkmarble/marble-backend/models"
)

type HTTPOpenSanctionsResult struct {
	Responses map[string]struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Results []json.RawMessage `json:"results"`
	} `json:"responses"`
}

type HTTPOpenSanctionResultResult struct {
	Id         string   `json:"id"`
	Schema     string   `json:"schema"`
	Datasets   []string `json:"datasets"`
	Properties struct {
		Name []string `json:"name"`
	} `json:"properties"`
}

func AdaptOpenSanctionsResult(query json.RawMessage, result HTTPOpenSanctionsResult) (models.SanctionCheckWithMatches, error) {
	partial := false
	matches := make(map[string]models.SanctionCheckMatch)
	matchToQueryId := make(map[string][]string)

	for queryId, resp := range result.Responses {
		if resp.Total.Value > len(resp.Results) {
			partial = true
		}

		for _, match := range resp.Results {
			var parsed HTTPOpenSanctionResultResult

			if err := json.NewDecoder(bytes.NewReader(match)).Decode(&parsed); err != nil {
				return models.SanctionCheckWithMatches{}, err
			}

			if _, ok := matches[parsed.Id]; !ok {
				entity := models.SanctionCheckMatch{
					Payload:  match,
					EntityId: parsed.Id,
				}

				matches[parsed.Id] = entity
			}

			matchToQueryId[parsed.Id] = append(matchToQueryId[parsed.Id], queryId)
		}

		for entityId, queryIds := range matchToQueryId {
			result := matches[entityId]
			result.QueryIds = queryIds

			matches[entityId] = result
		}
	}

	output := models.SanctionCheckWithMatches{
		SanctionCheck: models.SanctionCheck{
			Query:   query,
			Partial: partial,
		},
		Count:   len(matches),
		Matches: slices.Collect(maps.Values(matches)),
	}

	return output, nil
}
