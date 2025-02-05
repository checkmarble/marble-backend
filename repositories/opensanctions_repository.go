package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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

func (repo OpenSanctionsRepository) GetLatestUpstreamDataset(ctx context.Context) (models.OpenSanctionsUpstreamDataset, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OPEN_SANCTIONS_DATASET_URL, nil)
	if err != nil {
		return models.OpenSanctionsUpstreamDataset{}, err
	}

	resp, err := repo.opensanctions.Client().Do(req)
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

func (repo OpenSanctionsRepository) GetLatestLocalDataset(ctx context.Context) (models.OpenSanctionsDataset, error) {
	upstream, err := repo.GetLatestUpstreamDataset(ctx)
	if err != nil {
		return models.OpenSanctionsDataset{}, errors.Wrap(err, "could not retrieve upstream dataset")
	}

	u := fmt.Sprintf("%s/catalog", repo.opensanctions.Host())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return models.OpenSanctionsDataset{}, err
	}

	resp, err := repo.opensanctions.Client().Do(req)
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
	if err := dataset.CheckIsUpToDate(time.Now); err != nil {
		return models.OpenSanctionsDataset{}, nil
	}

	return dataset, nil
}

func (repo OpenSanctionsRepository) Search(ctx context.Context, query models.OpenSanctionsQuery) (models.SanctionCheckWithMatches, error) {
	req, rawQuery, err := repo.searchRequest(ctx, query)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	utils.LoggerFromContext(ctx).InfoContext(ctx, "sending sanction check query")
	startedAt := time.Now()

	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		return models.SanctionCheckWithMatches{},
			errors.Wrap(err, "could not perform sanction check")
	}

	if resp.StatusCode != http.StatusOK {
		return models.SanctionCheckWithMatches{}, fmt.Errorf(
			"sanction check API returned status %d", resp.StatusCode)
	}

	var matches httpmodels.HTTPOpenSanctionsResult

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&matches); err != nil {
		return models.SanctionCheckWithMatches{}, errors.Wrap(err,
			"could not parse sanction check response")
	}

	sanctionCheck, err := httpmodels.AdaptOpenSanctionsResult(rawQuery, matches)
	if err != nil {
		return sanctionCheck, err
	}

	utils.LoggerFromContext(ctx).InfoContext(ctx, "got successful sanction check response",
		"duration", time.Since(startedAt).Milliseconds(), "matches", len(sanctionCheck.Matches), "partial", sanctionCheck.Partial)

	return sanctionCheck, err
}

func (repo OpenSanctionsRepository) searchRequest(ctx context.Context,
	query models.OpenSanctionsQuery,
) (*http.Request, []byte, error) {
	q := openSanctionsRequest{
		Queries: make(map[string]openSanctionsRequestQuery, len(query.Queries)),
	}

	if !query.IsRefinement {
		for key, value := range query.Queries {
			q.Queries[uuid.NewString()] = openSanctionsRequestQuery{
				Schema:     "Thing",
				Properties: map[string][]string{key: value},
			}
		}
	} else {
		q.Queries[uuid.NewString()] = openSanctionsRequestQuery{
			Schema:     query.Type,
			Properties: query.Queries,
		}
	}

	var body, rawQuery bytes.Buffer

	if err := json.NewEncoder(io.MultiWriter(&body, &rawQuery)).Encode(q); err != nil {
		return nil, nil, errors.Wrap(err,
			"could not parse OpenSanctions response")
	}

	requestUrl := fmt.Sprintf("%s/match/sanctions", repo.opensanctions.Host())

	if qs := repo.buildQueryString(query.Config, query.OrgConfig); len(qs) > 0 {
		requestUrl = fmt.Sprintf("%s?%s", requestUrl, qs.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestUrl, &body)

	if repo.opensanctions.IsSelfHosted() {
		switch repo.opensanctions.AuthMethod() {
		case infra.OPEN_SANCTIONS_AUTH_BEARER:
			req.Header.Set("authorization", "Bearer "+repo.opensanctions.Credentials())
		case infra.OPEN_SANCTIONS_AUTH_BASIC:
			u, p, _ := strings.Cut(repo.opensanctions.Credentials(), ":")

			req.SetBasicAuth(u, p)
		}
	}

	return req, rawQuery.Bytes(), err
}

func (repo OpenSanctionsRepository) buildQueryString(cfg models.SanctionCheckConfig, orgCfg models.OrganizationOpenSanctionsConfig) url.Values {
	qs := url.Values{}

	if repo.opensanctions.AuthMethod() == infra.OPEN_SANCTIONS_AUTH_SAAS &&
		len(repo.opensanctions.Credentials()) > 0 {
		qs.Set("api_key", repo.opensanctions.Credentials())
	}

	if len(cfg.Datasets) > 0 {
		qs["include_dataset"] = cfg.Datasets
	}

	qs.Set("threshold", fmt.Sprintf("%.1f", float64(orgCfg.MatchThreshold)/100))
	qs.Set("limit", fmt.Sprintf("%d", orgCfg.MatchLimit))

	return qs
}
