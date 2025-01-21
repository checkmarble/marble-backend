package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

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

func (repo OpenSanctionsRepository) Search(ctx context.Context,
	cfg models.SanctionCheckConfig,
	query models.OpenSanctionsQuery,
) (models.SanctionCheckExecution, error) {
	req, queryPayload, err := repo.searchRequest(ctx, query)
	if err != nil {
		return models.SanctionCheckExecution{}, err
	}

	utils.LoggerFromContext(ctx).Debug("SANCTION CHECK: sending request...")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.SanctionCheckExecution{},
			errors.Wrap(err, "could not perform sanction check")
	}

	if resp.StatusCode != http.StatusOK {
		return models.SanctionCheckExecution{}, fmt.Errorf(
			"sanction check API returned status %d", resp.StatusCode)
	}

	var matches httpmodels.HTTPOpenSanctionsResult

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&matches); err != nil {
		return models.SanctionCheckExecution{}, errors.Wrap(err,
			"could not parse sanction check response")
	}

	var payload bytes.Buffer

	if err := json.NewEncoder(&payload).Encode(queryPayload); err != nil {
		return models.SanctionCheckExecution{}, errors.Wrap(err, "could not encode query")
	}

	query.QueryPayload = payload.Bytes()

	return httpmodels.AdaptOpenSanctionsResult(query, matches)
}

func (repo OpenSanctionsRepository) searchRequest(ctx context.Context,
	query models.OpenSanctionsQuery,
) (*http.Request, openSanctionsRequest, error) {
	q := openSanctionsRequest{
		Queries: make(map[string]openSanctionsRequestQuery, len(query.Queries)),
	}

	for key, value := range query.Queries {
		q.Queries[uuid.NewString()] = openSanctionsRequestQuery{
			Schema:     "Thing",
			Properties: map[string][]string{key: value},
		}
	}

	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(q); err != nil {
		return nil, openSanctionsRequest{}, errors.Wrap(err,
			"could not parse OpenSanctions response")
	}

	requestUrl := fmt.Sprintf("%s/match/sanctions", repo.opensanctions.Host())

	if qs := repo.buildQueryString(query.OrgConfig); len(qs) > 0 {
		requestUrl = fmt.Sprintf("%s?%s", requestUrl, qs.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestUrl, &body)

	return req, q, err
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
