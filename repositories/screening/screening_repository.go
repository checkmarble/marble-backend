package screening

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
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

// HTTPError represents an HTTP error from the screening API with the status code
type HTTPError struct {
	StatusCode int
	Message    string
}

type ScreeningProvider interface {
	BuildQueryString(ctx context.Context, cfg *models.ScreeningConfig, query *models.OpenSanctionsQuery) url.Values
	SearchRequest(ctx context.Context, query *models.OpenSanctionsQuery) (*http.Request, []byte, error)
	FindAvailableFilters(ctx context.Context) (dto.ScreeningAvailableFilters, error)
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("screening API returned status %d: %s", e.StatusCode, e.Message)
}

// IsTransient returns true if this is a transient error that should trigger a retry
func (e *HTTPError) IsTransient() bool {
	return e.StatusCode == http.StatusRequestTimeout || // 408
		e.StatusCode == http.StatusTooManyRequests || // 429
		e.StatusCode == http.StatusBadGateway || // 502
		e.StatusCode == http.StatusServiceUnavailable || // 503
		e.StatusCode == http.StatusGatewayTimeout // 504
}

type OpenSanctionsRepository struct {
	Config infra.Screening
}

type openSanctionsRequest struct {
	Queries map[string]openSanctionsRequestQuery `json:"queries"`
	Params  *motivaRequestParams                 `json:"params,omitempty"`
}

type motivaRequestParams struct {
	IncludeDatasets  []string `json:"include_datasets"`
	ExcludeDatasets  []string `json:"exclude_datasets"`
	ExcludeEntityIds []string `json:"exclude_entity_ids"`
}

type openSanctionsRequestQuery struct {
	Schema     string                     `json:"schema"`
	Properties models.OpenSanctionsFilter `json:"properties"`
	Filters    map[string][][]string      `json:"filters"`
}

func (repo OpenSanctionsRepository) IsSelfHosted(ctx context.Context) bool {
	return repo.Config.IsSelfHosted("opensanctions")
}

func (repo OpenSanctionsRepository) IsConfigured(ctx context.Context, provider string) (bool, error) {
	if ok, err := repo.Config.IsConfigured(provider); !ok {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"open sanction is not misconfigured", "error", err)

		return false, models.MissingRequirementError{
			Requirement: models.REQUIREMENT_OPEN_SANCTIONS,
			Reason:      models.REQUIREMENT_REASON_MISSING_CONFIGURATION,
			Err:         err,
		}
	}

	catalogUrl := fmt.Sprintf("%s/readyz", repo.Config.Host(provider))

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

	resp, err := repo.Config.Client().Do(req)
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

func (repo OpenSanctionsRepository) GetRawCatalog(ctx context.Context, provider string) (models.OpenSanctionsRawCatalog, error) {
	catalogUrl := fmt.Sprintf("%s/catalog", repo.Config.Host(provider))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogUrl, nil)
	if err != nil {
		return models.OpenSanctionsRawCatalog{}, err
	}
	resp, err := repo.Config.Client().Do(req)
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

func (repo OpenSanctionsRepository) GetCatalog(ctx context.Context, provider string) (models.OpenSanctionsCatalog, error) {
	cacheKey := fmt.Sprintf("%s:%s", OPEN_SANCTIONS_CATALOG_CACHE_KEY, provider)

	if cached, ok := OPEN_SANCTIONS_DATASET_CACHE.Get(cacheKey); ok {
		return cached, nil
	}

	catalogUrl := fmt.Sprintf("%s/catalog", repo.Config.Host(provider))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogUrl, nil)
	if err != nil {
		return models.OpenSanctionsCatalog{}, err
	}

	resp, err := repo.Config.Client().Do(req)
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
		OPEN_SANCTIONS_DATASET_CACHE.Add(cacheKey, catalogModel)
	}

	return catalogModel, err
}

func (repo OpenSanctionsRepository) GetLatestUpstreamDatasetFreshness(ctx context.Context) (models.OpenSanctionsUpstreamDatasetFreshness, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, OPEN_SANCTIONS_DEFAULT_INDEX_URL, nil)
	if err != nil {
		return models.OpenSanctionsUpstreamDatasetFreshness{}, err
	}

	resp, err := repo.Config.Client().Do(req)
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

	resp, err := repo.Config.Client().Do(req)
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

	u := fmt.Sprintf("%s/catalog", repo.Config.Host("opensanctions"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return models.OpenSanctionsDatasetFreshness{}, err
	}

	resp, err := repo.Config.Client().Do(req)
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
		fmt.Sprintf("%s/algorithms", repo.Config.Host("opensanctions")), nil)
	if err != nil {
		return models.OpenSanctionAlgorithms{}, err
	}

	resp, err := repo.Config.Client().Do(req)
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

func (repo OpenSanctionsRepository) GetProvider(provider string) ScreeningProvider {
	switch provider {
	case "lexisnexis":
		return ScreeningLexisNexisProvider(repo)
	default:
		return ScreeningOpenSanctionsProvider(repo)
	}
}

func (repo OpenSanctionsRepository) Search(ctx context.Context, providerName string, query models.OpenSanctionsQuery) (models.ScreeningRawSearchResponseWithMatches, error) {
	provider := repo.GetProvider(providerName)

	ctx, span := utils.OpenTelemetryTracerFromContext(ctx).Start(ctx, "yente-request")
	defer span.End()

	req, rawQuery, err := provider.SearchRequest(ctx, &query)
	if err != nil {
		return models.ScreeningRawSearchResponseWithMatches{}, err
	}

	repo.authenticateRequest(req)

	startedAt := time.Now()

	resp, err := repo.Config.Client().Do(req)

	span.End()

	if err != nil {
		return models.ScreeningRawSearchResponseWithMatches{},
			errors.Wrap(err, "could not perform screening")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.ScreeningRawSearchResponseWithMatches{}, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    "screening API error",
		}
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
		DebugContext(ctx, "got successful screening response",
			"duration", time.Since(startedAt).Milliseconds(),
			"matches", len(screening.Matches),
			"partial", screening.Partial)

	screening.EffectiveThreshold = query.EffectiveThreshold

	return screening, err
}

func (repo OpenSanctionsRepository) EnrichMatch(ctx context.Context, providerName string, match models.ScreeningMatch) ([]byte, error) {
	provider := repo.GetProvider(providerName)

	requestUrl := fmt.Sprintf("%s/entities/%s", repo.Config.Host(providerName), match.EntityId)

	if qs := provider.BuildQueryString(ctx, nil, nil); len(qs) > 0 {
		requestUrl = fmt.Sprintf("%s?%s", requestUrl, qs.Encode())
	}

	req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
	if err != nil {
		return nil, err
	}

	repo.authenticateRequest(req)

	resp, err := repo.Config.Client().Do(req)
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
	if repo.Config.IsSelfHosted("opensanctions") {
		switch repo.Config.AuthMethod() {
		case infra.SCREENING_AUTH_BEARER:
			req.Header.Set("authorization", "Bearer "+repo.Config.Credentials())
		case infra.SCREENING_AUTH_BASIC:
			u, p, _ := strings.Cut(repo.Config.Credentials(), ":")

			req.SetBasicAuth(u, p)
		}
	}
}

func (repo OpenSanctionsRepository) FindAvailableFilters(ctx context.Context, providerName string) (dto.ScreeningAvailableFilters, error) {
	provider := repo.GetProvider(providerName)

	return provider.FindAvailableFilters(ctx)
}
