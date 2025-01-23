package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const OPEN_SANCTIONS_DATASET_URL = "https://data.opensanctions.org/datasets/latest/default/index.json"

type OpenSanctionsRepository struct {
	opensanctions infra.OpenSanctions
}

type openSanctionsRequest struct {
	Queries map[string]openSanctionsRequestQuery `json:"queries"`
}

type openSanctionsRequestQuery struct {
	Schema     string                         `json:"schema"`
	Properties models.OpenSanctionCheckFilter `json:"properties"`
}

func (repo OpenSanctionsRepository) GetLatestUpstreamDataset(ctx context.Context,
	client http.Client,
) (models.OpenSanctionsUpstreamDataset, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OPEN_SANCTIONS_DATASET_URL, nil)
	if err != nil {
		return models.OpenSanctionsUpstreamDataset{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return models.OpenSanctionsUpstreamDataset{}, err
	}

	defer resp.Body.Close()

	var dataset httpmodels.HTTPOpenSanctionRemoteDataset

	if err := json.NewDecoder(resp.Body).Decode(&dataset); err != nil {
		return models.OpenSanctionsUpstreamDataset{}, err
	}

	return httpmodels.AdaptOpenSanctionDataset(dataset), err
}

func (repo OpenSanctionsRepository) GetLatestLocalDataset(ctx context.Context, client http.Client) (models.OpenSanctionsDataset, error) {
	upstream, err := repo.GetLatestUpstreamDataset(ctx, client)
	if err != nil {
		return models.OpenSanctionsDataset{}, errors.Wrap(err, "could not retrieve upstream dataset")
	}

	u := fmt.Sprintf("%s/catalog", repo.opensanctions.Host())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return models.OpenSanctionsDataset{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.OpenSanctionsDataset{}, err
	}

	defer resp.Body.Close()

	var localDataset httpmodels.HTTPOpenSanctionsLocalDatasets

	if err := json.NewDecoder(resp.Body).Decode(&localDataset); err != nil {
		return models.OpenSanctionsDataset{}, err
	}

	dataset, err := httpmodels.AdaptOpenSanctionsLocalDataset(localDataset)
	if err != nil {
		return models.OpenSanctionsDataset{}, errors.Wrap(err, "could not retrieve local dataset")
	}

	dataset.Upstream = upstream
	if err := dataset.CheckIsUpToDate(); err != nil {
		return models.OpenSanctionsDataset{}, nil
	}

	return dataset, nil
}

func (repo OpenSanctionsRepository) Search(ctx context.Context,
	cfg models.SanctionCheckConfig,
	query models.OpenSanctionsQuery,
) (models.SanctionCheck, error) {
	req, rawQuery, err := repo.searchRequest(ctx, query)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	utils.LoggerFromContext(ctx).Debug("SANCTION CHECK: sending request...")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.SanctionCheck{},
			errors.Wrap(err, "could not perform sanction check")
	}

	if resp.StatusCode != http.StatusOK {
		return models.SanctionCheck{}, fmt.Errorf(
			"sanction check API returned status %d", resp.StatusCode)
	}

	var matches httpmodels.HTTPOpenSanctionsResult

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&matches); err != nil {
		return models.SanctionCheck{}, errors.Wrap(err,
			"could not parse sanction check response")
	}

	return httpmodels.AdaptOpenSanctionsResult(rawQuery, matches)
}

func (repo OpenSanctionsRepository) searchRequest(ctx context.Context,
	query models.OpenSanctionsQuery,
) (*http.Request, []byte, error) {
	q := openSanctionsRequest{
		Queries: make(map[string]openSanctionsRequestQuery, len(query.Queries)),
	}

	for key, value := range query.Queries {
		q.Queries[uuid.NewString()] = openSanctionsRequestQuery{
			Schema:     "Thing",
			Properties: map[string][]string{key: value},
		}
	}

	var body, rawQuery bytes.Buffer

	if err := json.NewEncoder(io.MultiWriter(&body, &rawQuery)).Encode(q); err != nil {
		return nil, nil, errors.Wrap(err,
			"could not parse OpenSanctions response")
	}

	requestUrl := fmt.Sprintf("%s/match/sanctions", repo.opensanctions.Host())

	if qs := repo.buildQueryString(query.OrgConfig); len(qs) > 0 {
		requestUrl = fmt.Sprintf("%s?%s", requestUrl, qs.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestUrl, &body)

	return req, rawQuery.Bytes(), err
}

func (repo OpenSanctionsRepository) buildQueryString(orgCfg models.OrganizationOpenSanctionsConfig) url.Values {
	qs := url.Values{}

	if len(repo.opensanctions.ApiKey()) > 0 {
		qs.Set("api_key", repo.opensanctions.ApiKey())
	}

	if len(orgCfg.Datasets) > 0 {
		qs["include_dataset"] = orgCfg.Datasets
	}
	if orgCfg.MatchLimit != nil {
		qs.Set("limit", fmt.Sprintf("%d", *orgCfg.MatchLimit))
	}
	if orgCfg.MatchThreshold != nil {
		qs.Set("threshold", fmt.Sprintf("%.1f", float64(*orgCfg.MatchThreshold)/100))
	}

	return qs
}
