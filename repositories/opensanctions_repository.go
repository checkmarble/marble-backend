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
	"github.com/hashicorp/golang-lru/v2/expirable"
)

const (
	OPEN_SANCTIONS_DEFAULT_INDEX_URL      = "https://data.opensanctions.org/datasets/latest/default/index.json"
	OPEN_SANCTIONS_INDEX_URL              = "https://data.opensanctions.org/datasets/latest/index.json"
	OPEN_SANCTIONS_CATALOG_CACHE_KEY      = "catalog"
	OPEN_SANCTIONS_ALGORITHMS_CACHE_KEY   = "algorithms"
	OPEN_SANCTIONS_MAX_EXCLUDE_ENTITY_IDS = 50
)

var (
	OPEN_SANCTIONS_DATASET_CACHE    = expirable.NewLRU[string, models.OpenSanctionsCatalog](1, nil, time.Hour)
	OPEN_SANCTIONS_DATASET_TAGS     = expirable.NewLRU[string, []string](0, nil, 0)
	OPEN_SANCTIONS_ALGORITHMS_CACHE = expirable.NewLRU[string, models.OpenSanctionAlgorithms](1, nil, time.Hour)
)

type OpenSanctionsRepository struct {
	opensanctions infra.OpenSanctions
}

type openSanctionsRequest struct {
	Queries map[string]openSanctionsRequestQuery `json:"queries"`
}

type openSanctionsRequestQuery struct {
	Schema     string                     `json:"schema"`
	Properties models.OpenSanctionsFilter `json:"properties"`
}

func (repo OpenSanctionsRepository) IsSelfHosted(ctx context.Context) bool {
	return repo.opensanctions.IsSelfHosted()
}

func (repo OpenSanctionsRepository) IsConfigured(ctx context.Context) (bool, error) {
	if ok, err := repo.opensanctions.IsConfigured(); !ok {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"open sanction is not misconfigured", "error", err)

		return false, models.MissingRequirementError{
			Requirement: models.REQUIREMENT_OPEN_SANCTIONS,
			Reason:      models.REQUIREMENT_REASON_MISSING_CONFIGURATION,
			Err:         err,
		}
	}

	catalogUrl := fmt.Sprintf("%s/readyz", repo.opensanctions.Host())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogUrl, nil)
	if err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"could not create OpenSanctions healthcheck request", "error", err)

		return false, models.MissingRequirementError{
			Requirement: models.REQUIREMENT_OPEN_SANCTIONS,
			Reason:      models.REQUIREMENT_REASON_INVALID_CONFIGURATION,
			Err:         err,
		}
	}

	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"OpenSanctions healthcheck returned an error", "error", err)

		return false, models.MissingRequirementError{
			Requirement: models.REQUIREMENT_OPEN_SANCTIONS,
			Reason:      models.REQUIREMENT_REASON_HEALTHCHECK_FAILED,
			Err:         err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"OpenSanctions healthcheck returned non-OK status code", "code", resp.StatusCode)

		return false, models.MissingRequirementError{
			Requirement: models.REQUIREMENT_OPEN_SANCTIONS,
			Reason:      models.REQUIREMENT_REASON_HEALTHCHECK_FAILED,
			Err:         fmt.Errorf("healthcheck returned status code %d", resp.StatusCode),
		}
	}

	return true, nil
}

func (repo OpenSanctionsRepository) GetRawCatalog(ctx context.Context) (models.OpenSanctionsRawCatalog, error) {
	catalogUrl := fmt.Sprintf("%s/catalog", repo.opensanctions.Host())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogUrl, nil)
	if err != nil {
		return models.OpenSanctionsRawCatalog{}, err
	}
	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		return models.OpenSanctionsRawCatalog{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return models.OpenSanctionsRawCatalog{},
			fmt.Errorf("failed to get raw catalog: %d", resp.StatusCode)
	}

	var httpCatalog httpmodels.HTTPOpenSanctionCatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&httpCatalog); err != nil {
		return models.OpenSanctionsRawCatalog{}, err
	}

	return httpmodels.AdaptOpenSanctionCatalogResponse(httpCatalog), nil
}

func (repo OpenSanctionsRepository) GetCatalog(ctx context.Context) (models.OpenSanctionsCatalog, error) {
	if cached, ok := OPEN_SANCTIONS_DATASET_CACHE.Get(OPEN_SANCTIONS_CATALOG_CACHE_KEY); ok {
		return cached, nil
	}

	catalogUrl := fmt.Sprintf("%s/catalog", repo.opensanctions.Host())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogUrl, nil)
	if err != nil {
		return models.OpenSanctionsCatalog{}, err
	}

	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		return models.OpenSanctionsCatalog{}, err
	}

	defer resp.Body.Close()

	var catalog httpmodels.HTTPOpenSanctionCatalogResponse

	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return models.OpenSanctionsCatalog{}, err
	}

	if err := repo.GatherTags(ctx); err != nil {
		return models.OpenSanctionsCatalog{}, err
	}

	catalogModel := httpmodels.AdaptOpenSanctionCatalog(catalog.Datasets, OPEN_SANCTIONS_DATASET_TAGS)

	if len(catalogModel.Sections) > 0 {
		OPEN_SANCTIONS_DATASET_CACHE.Add(OPEN_SANCTIONS_CATALOG_CACHE_KEY, catalogModel)
	}

	return catalogModel, err
}

func (repo OpenSanctionsRepository) GetLatestUpstreamDatasetFreshness(ctx context.Context) (models.OpenSanctionsUpstreamDatasetFreshness, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OPEN_SANCTIONS_DEFAULT_INDEX_URL, nil)
	if err != nil {
		return models.OpenSanctionsUpstreamDatasetFreshness{}, err
	}

	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		return models.OpenSanctionsUpstreamDatasetFreshness{}, err
	}

	defer resp.Body.Close()

	var dataset httpmodels.HTTPOpenSanctionRemoteDataset

	if err := json.NewDecoder(resp.Body).Decode(&dataset); err != nil {
		return models.OpenSanctionsUpstreamDatasetFreshness{}, err
	}

	return httpmodels.AdaptOpenSanctionDatasetFreshness(dataset), err
}

func (repo OpenSanctionsRepository) GatherTags(ctx context.Context) error {
	if OPEN_SANCTIONS_DATASET_TAGS.Len() > 0 {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OPEN_SANCTIONS_INDEX_URL, nil)
	if err != nil {
		return err
	}

	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var dataset httpmodels.HTTPOpenSanctionsRemoteIndexTags

	if err := json.NewDecoder(resp.Body).Decode(&dataset); err != nil {
		return err
	}

	for _, ds := range dataset.Datasets {
		OPEN_SANCTIONS_DATASET_TAGS.Add(ds.Name, ds.Tags)
	}

	return nil
}

func (repo OpenSanctionsRepository) GetLatestLocalDataset(ctx context.Context) (models.OpenSanctionsDatasetFreshness, error) {
	upstream, err := repo.GetLatestUpstreamDatasetFreshness(ctx)
	if err != nil {
		return models.OpenSanctionsDatasetFreshness{},
			errors.Wrap(err, "could not retrieve upstream dataset")
	}

	u := fmt.Sprintf("%s/catalog", repo.opensanctions.Host())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return models.OpenSanctionsDatasetFreshness{}, err
	}

	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		return models.OpenSanctionsDatasetFreshness{}, err
	}

	defer resp.Body.Close()

	var localDataset httpmodels.HTTPOpenSanctionsLocalDatasets

	if err := json.NewDecoder(resp.Body).Decode(&localDataset); err != nil {
		return models.OpenSanctionsDatasetFreshness{}, err
	}

	dataset, err := httpmodels.AdaptOpenSanctionsLocalDatasetFreshness(localDataset)
	if err != nil {
		return models.OpenSanctionsDatasetFreshness{},
			errors.Wrap(err, "could not retrieve local dataset")
	}

	dataset.Upstream = upstream
	if err := dataset.CheckIsUpToDate(time.Now); err != nil {
		return models.OpenSanctionsDatasetFreshness{}, err
	}

	return dataset, nil
}

func (repo OpenSanctionsRepository) GetAlgorithms(ctx context.Context) (models.OpenSanctionAlgorithms, error) {
	if cached, ok := OPEN_SANCTIONS_ALGORITHMS_CACHE.Get(OPEN_SANCTIONS_ALGORITHMS_CACHE_KEY); ok {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/algorithms", repo.opensanctions.Host()), nil)
	if err != nil {
		return models.OpenSanctionAlgorithms{}, err
	}

	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		return models.OpenSanctionAlgorithms{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.OpenSanctionAlgorithms{},
			fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var algorithms httpmodels.HTTPOpenSanctionsAlgorithms
	if err := json.NewDecoder(resp.Body).Decode(&algorithms); err != nil {
		return models.OpenSanctionAlgorithms{}, err
	}

	modelAlgorithms := httpmodels.AdaptOpenSanctionsAlgorithms(algorithms)
	OPEN_SANCTIONS_ALGORITHMS_CACHE.Add(OPEN_SANCTIONS_ALGORITHMS_CACHE_KEY, modelAlgorithms)
	return modelAlgorithms, nil
}

func (repo OpenSanctionsRepository) Search(ctx context.Context, query models.OpenSanctionsQuery) (models.ScreeningRawSearchResponseWithMatches, error) {
	ctx, span := utils.OpenTelemetryTracerFromContext(ctx).Start(ctx, "yente-request")
	defer span.End()

	req, rawQuery, err := repo.searchRequest(ctx, &query)
	if err != nil {
		return models.ScreeningRawSearchResponseWithMatches{}, err
	}

	utils.LoggerFromContext(ctx).InfoContext(ctx, "sending screening query")
	startedAt := time.Now()

	resp, err := repo.opensanctions.Client().Do(req)

	span.End()

	if err != nil {
		return models.ScreeningRawSearchResponseWithMatches{},
			errors.Wrap(err, "could not perform screening")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.ScreeningRawSearchResponseWithMatches{}, fmt.Errorf(
			"screening API returned status %d", resp.StatusCode)
	}

	var matches httpmodels.HTTPOpenSanctionsResult

	if err := json.NewDecoder(resp.Body).Decode(&matches); err != nil {
		return models.ScreeningRawSearchResponseWithMatches{}, errors.Wrap(err,
			"could not parse screening response")
	}

	screening, err := httpmodels.AdaptOpenSanctionsResult(rawQuery, matches)
	if err != nil {
		return screening, err
	}

	utils.LoggerFromContext(ctx).
		InfoContext(ctx, "got successful screening response",
			"duration", time.Since(startedAt).Milliseconds(),
			"matches", len(screening.Matches),
			"partial", screening.Partial)

	screening.EffectiveThreshold = query.EffectiveThreshold

	return screening, err
}

func (repo OpenSanctionsRepository) EnrichMatch(ctx context.Context, match models.ScreeningMatch) ([]byte, error) {
	requestUrl := fmt.Sprintf("%s/entities/%s", repo.opensanctions.Host(), match.EntityId)

	if qs := repo.buildQueryString(nil, nil); len(qs) > 0 {
		requestUrl = fmt.Sprintf("%s?%s", requestUrl, qs.Encode())
	}

	req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		return nil, err
	}

	repo.authenticateRequest(req)

	resp, err := repo.opensanctions.Client().Do(req)
	if err != nil {
		return nil,
			errors.Wrap(err, "could not enrich screening match")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.WithDetail(models.NotFoundError, "an entity with this ID was not found")
		}

		return nil, fmt.Errorf(
			"screening API returned status %d on enrichment", resp.StatusCode)
	}

	var newMatch json.RawMessage

	if err := json.NewDecoder(resp.Body).Decode(&newMatch); err != nil {
		return nil, errors.Wrap(err,
			"could not parse screening response")
	}

	return newMatch, nil
}

func (repo OpenSanctionsRepository) authenticateRequest(req *http.Request) {
	if repo.opensanctions.IsSelfHosted() {
		switch repo.opensanctions.AuthMethod() {
		case infra.OPEN_SANCTIONS_AUTH_BEARER:
			req.Header.Set("authorization", "Bearer "+repo.opensanctions.Credentials())
		case infra.OPEN_SANCTIONS_AUTH_BASIC:
			u, p, _ := strings.Cut(repo.opensanctions.Credentials(), ":")

			req.SetBasicAuth(u, p)
		}
	}
}

func (repo OpenSanctionsRepository) searchRequest(ctx context.Context,
	query *models.OpenSanctionsQuery,
) (*http.Request, []byte, error) {
	q := openSanctionsRequest{
		Queries: make(map[string]openSanctionsRequestQuery, len(query.Queries)),
	}

	for _, subquery := range query.Queries {
		q.Queries[uuid.NewString()] = openSanctionsRequestQuery{
			Schema:     subquery.Type,
			Properties: subquery.Filters,
		}
	}

	scope := repo.opensanctions.Scope()
	if query.Scope != "" {
		scope = query.Scope
	}

	var body, rawQuery bytes.Buffer

	if err := json.NewEncoder(io.MultiWriter(&body, &rawQuery)).Encode(q); err != nil {
		return nil, nil, errors.Wrap(err,
			"could not parse OpenSanctions response")
	}

	requestUrl := fmt.Sprintf("%s/match/%s", repo.opensanctions.Host(), scope)

	if qs := repo.buildQueryString(&query.Config, query); len(qs) > 0 {
		requestUrl = fmt.Sprintf("%s?%s", requestUrl, qs.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestUrl, &body)
	req.Header.Set("content-type", "application/json")

	repo.authenticateRequest(req)

	return req, rawQuery.Bytes(), err
}

func (repo OpenSanctionsRepository) buildQueryString(cfg *models.ScreeningConfig, query *models.OpenSanctionsQuery) url.Values {
	qs := url.Values{}

	if repo.opensanctions.AuthMethod() == infra.OPEN_SANCTIONS_AUTH_SAAS &&
		len(repo.opensanctions.Credentials()) > 0 {
		qs.Set("api_key", repo.opensanctions.Credentials())
	}

	if cfg != nil && len(cfg.Datasets) > 0 {
		qs["include_dataset"] = cfg.Datasets
	}

	qs.Set("algorithm", repo.opensanctions.Algorithm())

	if query != nil {
		query.EffectiveThreshold = utils.Or(query.Config.Threshold, query.OrgConfig.MatchThreshold)

		// Unless determined otherwise, we do not need those results that are *not*
		// matches. They could still be filtered further down the chain, but we do not need them returned.
		qs.Set("threshold", fmt.Sprintf("%.2f", float64(query.EffectiveThreshold)/100))
		qs.Set("cutoff", fmt.Sprintf("%.2f", float64(query.OrgConfig.MatchThreshold)/100))

		qs.Set("limit", fmt.Sprintf("%d", query.OrgConfig.MatchLimit+query.LimitIncrease))

		// cf: `exclude_entity_ids` in the OpenSanctions query
		// cf: https://api.opensanctions.org/#tag/Matching/operation/match_match__dataset__post
		// exclude_entity_ids is a global filter that is applied to all queries and the list is limited to 50 elements.
		// For our use, we think this is enough, in case we need to add more, we need to think about how to handle it.
		if len(query.WhitelistedEntityIds) > 0 {
			qs["exclude_entity_ids"] = query.WhitelistedEntityIds[:min(
				OPEN_SANCTIONS_MAX_EXCLUDE_ENTITY_IDS, len(query.WhitelistedEntityIds))]
		}
	}

	return qs
}
