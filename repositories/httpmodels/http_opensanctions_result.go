package httpmodels

import (
	"maps"
	"slices"

	"github.com/checkmarble/marble-backend/models"
)

type HTTPOpenSanctionsResult struct {
	Responses map[string]struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Results []struct {
			Id         string   `json:"id"`
			Schema     string   `json:"schema"`
			Datasets   []string `json:"datasets"`
			Properties struct {
				Name []string `json:"name"`
			} `json:"properties"`
		} `json:"results"`
	} `json:"responses"`
}

func AdaptOpenSanctionsResult(result HTTPOpenSanctionsResult) (models.SanctionCheckExecution, error) {
	// TODO: Replace with actual processing of responses
	partial := false
	matches := make(map[string]models.SanctionCheckExecutionMatch)

	for _, resp := range result.Responses {
		if resp.Total.Value > len(resp.Results) {
			partial = true
		}

		for _, match := range resp.Results {
			if _, ok := matches[match.Id]; !ok {
				entity := models.SanctionCheckExecutionMatch{
					Id:       match.Id,
					Schema:   match.Schema,
					Datasets: match.Datasets,
					Names:    match.Properties.Name,
				}

				matches[match.Id] = entity
			}
		}
	}

	output := models.SanctionCheckExecution{
		Partial: partial,
		Count:   len(matches),
		Matches: slices.Collect(maps.Values(matches)),
	}

	return output, nil
}
