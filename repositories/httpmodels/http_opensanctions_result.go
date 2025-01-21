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

func AdaptOpenSanctionsResult(query models.OpenSanctionsQuery, result HTTPOpenSanctionsResult) (models.SanctionCheckExecution, error) {
	// TODO: Replace with actual processing of responses
	partial := false
	matches := make(map[string]models.SanctionCheckExecutionMatch)
	matchToQueryId := make(map[string][]string)

	for queryId, resp := range result.Responses {
		if resp.Total.Value > len(resp.Results) {
			partial = true
		}

		for _, match := range resp.Results {
			var parsed HTTPOpenSanctionResultResult

			if err := json.NewDecoder(bytes.NewReader(match)).Decode(&parsed); err != nil {
				return models.SanctionCheckExecution{}, err
			}

			if _, ok := matches[parsed.Id]; !ok {
				entity := models.SanctionCheckExecutionMatch{
					Payload:  match,
					EntityId: parsed.Id,
					Datasets: parsed.Datasets,
				}

				matchToQueryId[parsed.Id] = append(matchToQueryId[parsed.Id], queryId)

				matches[parsed.Id] = entity
			} else {
				matchToQueryId[parsed.Id] = append(matchToQueryId[parsed.Id], queryId)
			}
		}

		for entityId, queryIds := range matchToQueryId {
			result := matches[entityId]
			result.QueryIds = queryIds

			matches[entityId] = result
		}
	}

	output := models.SanctionCheckExecution{
		Query:   query,
		Partial: partial,
		Count:   len(matches),
		Matches: slices.Collect(maps.Values(matches)),
	}

	return output, nil
}
