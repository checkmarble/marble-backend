package httpmodels

import (
	"bytes"
	"cmp"
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
	Limit int `json:"limit"`
}

type HTTPOpenSanctionResultResult struct {
	Id         string   `json:"id"`
	Referents  []string `json:"referents"`
	Match      bool     `json:"match"`
	Schema     string   `json:"schema"`
	Datasets   []string `json:"datasets"`
	Score      float64  `json:"score"`
	Properties struct {
		Name []string `json:"name"`
	} `json:"properties"`
}

// AdaptOpenSanctionsResult merges all subquery responses into a single deduplicated,
// score-ordered match list, then truncates it to the provider-echoed limit.
//
// In Marble, all subqueries within the same match request serve a single screening
// purpose — they are linked variants of the same lookup (e.g. Lexis Nexis topic
// fan-out, or Vehicle expanding to Airplane + Vessel). Merging and truncating their
// results to a single list is therefore correct and intentional.
//
// If two independent lookups need separate result lists, issue them as two distinct
// match requests instead of bundling them into one.
func AdaptOpenSanctionsResult(query json.RawMessage, result HTTPOpenSanctionsResult) (models.ScreeningRawSearchResponseWithMatches, error) {
	partial := false
	matches := make(map[string]models.ScreeningMatch)
	matchToQueryId := make(map[string][]string)

	for queryId, resp := range result.Responses {
		matchCount := 0

		for _, match := range resp.Results {
			var parsed HTTPOpenSanctionResultResult

			if err := json.NewDecoder(bytes.NewReader(match)).Decode(&parsed); err != nil {
				return models.ScreeningRawSearchResponseWithMatches{}, err
			}

			if !parsed.Match {
				continue
			}

			matchCount += 1

			if _, ok := matches[parsed.Id]; !ok {
				entity := models.ScreeningMatch{
					IsMatch:   parsed.Match,
					Payload:   match,
					EntityId:  parsed.Id,
					Referents: parsed.Referents,
					Score:     parsed.Score,
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

	sortedMatches := slices.SortedFunc(maps.Values(matches), func(m1, m2 models.ScreeningMatch) int {
		if n := cmp.Compare(m2.Score, m1.Score); n != 0 {
			return n
		}
		return cmp.Compare(m1.EntityId, m2.EntityId)
	})

	if result.Limit > 0 && len(sortedMatches) > result.Limit {
		sortedMatches = sortedMatches[:result.Limit]
	}

	output := models.ScreeningRawSearchResponseWithMatches{
		SearchInput:       query,
		Partial:           partial,
		InitialHasMatches: len(matches) > 0,

		Matches: sortedMatches,
		Count:   len(sortedMatches),
	}

	return output, nil
}
