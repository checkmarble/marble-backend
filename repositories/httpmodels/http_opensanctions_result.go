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
	Match      bool     `json:"match"`
	Schema     string   `json:"schema"`
	Datasets   []string `json:"datasets"`
	Properties struct {
		Name []string `json:"name"`
	} `json:"properties"`
}

func AdaptOpenSanctionsResult(query json.RawMessage, result HTTPOpenSanctionsResult) (models.SanctionRawSearchResponseWithMatches, error) {
	partial := false
	matches := make(map[string]models.SanctionCheckMatch)
	matchToQueryId := make(map[string][]string)

	for queryId, resp := range result.Responses {
		matchCount := 0

		for _, match := range resp.Results {
			var parsed HTTPOpenSanctionResultResult

			if err := json.NewDecoder(bytes.NewReader(match)).Decode(&parsed); err != nil {
				return models.SanctionRawSearchResponseWithMatches{}, err
			}

			if !parsed.Match {
				continue
			}

			matchCount += 1

			if _, ok := matches[parsed.Id]; !ok {
				entity := models.SanctionCheckMatch{
					IsMatch:  parsed.Match,
					Payload:  match,
					EntityId: parsed.Id,
				}

				matches[parsed.Id] = entity
			}

			matchToQueryId[parsed.Id] = append(matchToQueryId[parsed.Id], queryId)
		}

		// resp.Total.Value returns the total number of actual matches, regardless of what is returned.
		if resp.Total.Value > matchCount {
			partial = true
		}

		for entityId, queryIds := range matchToQueryId {
			result := matches[entityId]
			result.QueryIds = queryIds

			matches[entityId] = result
		}
	}

	output := models.SanctionRawSearchResponseWithMatches{
		SearchInput:       query,
		Partial:           partial,
		InitialHasMatches: len(matches) > 0,

		Matches: slices.Collect(maps.Values(matches)),
		Count:   len(matches),
	}

	return output, nil
}
