package httpmodels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/pkg/errors"
)

type HTTPOpenSanctionRemoteDataset struct {
	Name      string           `json:"name"`
	Version   string           `json:"version"`
	UpdatedAt OpenSanctionTime `json:"updated_at"`
	Coverage  struct {
		Schedule string `json:"schedule"`
	} `json:"coverage"`
}

type OpenSanctionTime time.Time

func (dt *OpenSanctionTime) UnmarshalJSON(b []byte) error {
	if b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("could not parse date as string")
	}

	s := string(b[1 : len(b)-1])

	d, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.UTC)
	if err != nil {
		return err
	}

	*dt = OpenSanctionTime(d)

	return nil
}

func AdaptOpenSanctionDataset(dataset HTTPOpenSanctionRemoteDataset) models.OpenSanctionsUpstreamDataset {
	return models.OpenSanctionsUpstreamDataset{
		Name:      dataset.Name,
		Version:   dataset.Version,
		UpdatedAt: time.Time(dataset.UpdatedAt),
		Schedule:  dataset.Coverage.Schedule,
	}
}

type HTTPOpenSanctionsLocalDatasets struct {
	Datasets []struct {
		Name         string `json:"name"`
		IndexVersion string `json:"index_version"`
	} `json:"datasets"`
}

func AdaptOpenSanctionsLocalDataset(datasets HTTPOpenSanctionsLocalDatasets) (models.OpenSanctionsDataset, error) {
	var version *string

	for _, ds := range datasets.Datasets {
		if ds.Name == "default" {
			version = &ds.IndexVersion
		}
	}

	if version == nil {
		return models.OpenSanctionsDataset{}, errors.New(
			"could not find upstream default dataset")
	}

	lastUpdatedAt, err := time.ParseInLocation("20060102150405", (*version)[:len(*version)-4], time.UTC)
	if err != nil {
		return models.OpenSanctionsDataset{}, errors.Wrap(err, "could not parse index time")
	}

	return models.OpenSanctionsDataset{
		Version:   *version,
		UpdatedAt: lastUpdatedAt,
	}, nil
}

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

func AdaptOpenSanctionsResult(query json.RawMessage, result HTTPOpenSanctionsResult) (models.SanctionCheck, error) {
	// TODO: Replace with actual processing of responses
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
				return models.SanctionCheck{}, err
			}

			if _, ok := matches[parsed.Id]; !ok {
				entity := models.SanctionCheckMatch{
					Payload:  match,
					EntityId: parsed.Id,
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

	output := models.SanctionCheck{
		Query:   query,
		Partial: partial,
		Count:   len(matches),
		Matches: slices.Collect(maps.Values(matches)),
	}

	return output, nil
}
