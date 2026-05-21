package screening

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type ScreeningLexisNexisProvider struct {
	Config infra.Screening
}

func (p ScreeningLexisNexisProvider) SearchRequest(ctx context.Context,
	query *models.OpenSanctionsQuery,
) (*http.Request, []byte, error) {
	q := openSanctionsRequest{
		Queries: make(map[string]openSanctionsRequestQuery, len(query.Queries)),
	}

	if p.Config.MotivaFeatures(ctx).BodyParams {
		q.Params = &motivaRequestParams{}

		if len(query.WhitelistedEntityIds) > 0 {
			q.Params.ExcludeEntityIds = query.WhitelistedEntityIds
		}
	}

	for _, subquery := range query.Queries {
		filters := query.Config.Filters.Resolve()

		for topic, filter := range filters.WithRootTopics() {
			if topic != "global" && (filters.NoFilters() || filter.IsEnabled()) {
				id := pure_utils.NewId().String()

				q.Queries[id] = openSanctionsRequestQuery{
					Schema:     subquery.Type,
					Properties: subquery.Filters,
					Filters: map[string][][]string{
						"topics": {{topic}},
					},
				}

				if filter.Datasets != nil {
					q.Queries[id].Filters["programId"] = [][]string{filter.Datasets}
				}

				for _, globalTopic := range filters.Global.Topics {
					q.Queries[id].Filters["topics"] = append(q.Queries[id].Filters["topics"], globalTopic)
				}

				if filter.Topics != nil {
					for _, topic := range filter.Topics {
						q.Queries[id].Filters["topics"] = append(q.Queries[id].Filters["topics"], topic)
					}
				}
			}
		}
	}

	scope := p.Config.Scope(models.ScreeningProviderLexisNexis)
	if query.Scope != "" {
		scope = query.Scope
	}

	var body, rawQuery bytes.Buffer

	if err := json.NewEncoder(io.MultiWriter(&body, &rawQuery)).Encode(q); err != nil {
		return nil, nil, errors.Wrap(err,
			"could not parse OpenSanctions response")
	}

	requestUrl := fmt.Sprintf("%s/match/%s", p.Config.Host(models.ScreeningProviderLexisNexis), scope)

	if qs := p.BuildQueryString(ctx, &query.Config, query); len(qs) > 0 {
		requestUrl = fmt.Sprintf("%s?%s", requestUrl, qs.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestUrl, &body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not create screening request")
	}

	req.Header.Set("content-type", "application/json")

	return req, rawQuery.Bytes(), err
}

func (p ScreeningLexisNexisProvider) BuildQueryString(ctx context.Context,
	cfg *models.ScreeningConfig, query *models.OpenSanctionsQuery,
) url.Values {
	qs := url.Values{}

	if p.Config.AuthMethod() == infra.SCREENING_AUTH_SAAS &&
		len(p.Config.Credentials()) > 0 {
		qs.Set("api_key", p.Config.Credentials())
	}

	if !p.Config.MotivaFeatures(ctx).BodyParams && cfg != nil && len(cfg.Datasets) > 0 {
		qs["include_dataset"] = []string{"lexisnexis"}
	}

	qs.Set("algorithm", p.Config.Algorithm())

	if query != nil {
		if p.Config.MotivaFeatures(ctx).ScopedIndex && query.UseScopedIndex {
			qs.Set("index_type", "scoped")
		}

		query.EffectiveThreshold = utils.Or(query.Config.Threshold, query.OrgConfig.MatchThreshold)

		// Unless determined otherwise, we do not need those results that are *not*
		// matches. They could still be filtered further down the chain, but we do not need them returned.
		qs.Set("threshold", fmt.Sprintf("%.2f", float64(query.EffectiveThreshold)/100))
		qs.Set("cutoff", fmt.Sprintf("%.2f", float64(query.EffectiveThreshold)/100))

		if query.LimitOverride != nil {
			qs.Set("limit", fmt.Sprintf("%d", *query.LimitOverride))
		} else {
			qs.Set("limit", fmt.Sprintf("%d", query.OrgConfig.MatchLimit))
		}

		// cf: `exclude_entity_ids` in the OpenSanctions query
		// cf: https://api.opensanctions.org/#tag/Matching/operation/match_match__dataset__post
		// exclude_entity_ids is a global filter that is applied to all queries and the list is limited to 50 elements.
		// For our use, we think this is enough, in case we need to add more, we need to think about how to handle it.
		if !p.Config.MotivaFeatures(ctx).BodyParams && len(query.WhitelistedEntityIds) > 0 {
			qs["exclude_entity_ids"] = query.WhitelistedEntityIds[:min(
				OPEN_SANCTIONS_MAX_EXCLUDE_ENTITY_IDS, len(query.WhitelistedEntityIds))]
		}
	}

	return qs
}

type lexisNexisAvailableFilters struct {
	Datasets []string `json:"properties.programId"` //nolint:tagliatelle
	Topics   []string `json:"properties.topics"`    //nolint:tagliatelle
}

func (p ScreeningLexisNexisProvider) FindAvailableFilters(ctx context.Context) (dto.ScreeningAvailableFilters, error) {
	catalog, err := p.GetLexisNexisCatalog(ctx)
	if err != nil {
		return dto.ScreeningAvailableFilters{}, err
	}

	url := fmt.Sprintf("%s/catalog/fields", p.Config.Host(models.ScreeningProviderLexisNexis))

	payload := map[string]any{
		"fields": []string{"properties.programId", "properties.topics"},
		"query": map[string]any{
			"term": map[string]any{
				"datasets": "lexisnexis",
			},
		},
	}

	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return dto.ScreeningAvailableFilters{}, err
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return dto.ScreeningAvailableFilters{}, errors.Wrap(err, "could not create available filters request")
	}

	req.Header.Set("content-type", "application/json")

	resp, err := p.Config.Client().Do(req)
	if err != nil {
		return dto.ScreeningAvailableFilters{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return dto.ScreeningAvailableFilters{}, errors.Newf("could not retrieve field values, got status code %d", resp.StatusCode)
	}

	var values lexisNexisAvailableFilters

	if err := json.NewDecoder(resp.Body).Decode(&values); err != nil {
		return dto.ScreeningAvailableFilters{}, err
	}

	filters := dto.ScreeningAvailableFilters{
		Provider: models.ScreeningProviderLexisNexis,
		Sections: dto.ScreeningAvailableFiltersSections{
			Sanctions: dto.ScreeningAvailableFiltersSection{
				Self: "sanctions",
				Datasets: pure_utils.Map(values.Datasets, func(ds string) dto.ScreeningAvailableFiltersItem {
					regionCode, _ := httpmodels.RegionFromDatasetName(ds)

					name := ds
					if datasets, ok := catalog.Metadata["datasets"].(map[string]any); ok {
						if dsName, ok := datasets[ds]; ok {
							if dsNameString, ok := dsName.(string); ok {
								name = dsNameString
							}
						}
					}

					return dto.ScreeningAvailableFiltersItem{
						Section: regionCode,
						Name:    ds,
						Title:   name,
					}
				}),
			},
			// TODO: add PEPs and adverse media topics, organized by kind.
			Peps: dto.ScreeningAvailableFiltersSection{
				Self: "pep",
				Topics: map[string][]dto.ScreeningAvailableFiltersItem{
					"status": {
						{Name: "pep.status.active", Title: "pep.status.active"},
						{Name: "pep.status.inactive", Title: "pep.status.inactive"},
					},
					"kind": {
						{Name: "pep.kind.primary", Title: "pep.kind.primary"},
						{Name: "pep.kind.secondary", Title: "pep.kind.secondary"},
					},
					"geography": {
						{Name: "pep.geo.eu", Title: "pep.geo.eu"},
						{Name: "pep.geo.us", Title: "pep.geo.us"},
					},
					"position": {
						{Name: "pep.position.headofstate", Title: "pep.position.headofstate"},
						{Name: "pep.position.legislative", Title: "pep.position.legislative"},
					},
				},
			},
			AdverseMedia: dto.ScreeningAvailableFiltersSection{
				Self: "adversemedia",
				Topics: map[string][]dto.ScreeningAvailableFiltersItem{
					"source": {
						{Name: "adversemedia.media", Title: "adversemedia.media"},
						{Name: "adversemedia.enforcements", Title: "adversemedia.enforcements"},
					},
				},
			},
		},
	}

	return filters, nil
}

func (p ScreeningLexisNexisProvider) GetLexisNexisCatalog(ctx context.Context) (httpmodels.HTTPOpenSanctionCatalogDataset, error) {
	catalogUrl := fmt.Sprintf("%s/catalog", p.Config.Host(models.ScreeningProviderLexisNexis))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogUrl, nil)
	if err != nil {
		return httpmodels.HTTPOpenSanctionCatalogDataset{}, err
	}

	resp, err := p.Config.Client().Do(req)
	if err != nil {
		return httpmodels.HTTPOpenSanctionCatalogDataset{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return httpmodels.HTTPOpenSanctionCatalogDataset{}, errors.Newf("got %d while fetching Lexis Nexis catalog", resp.StatusCode)
	}

	var catalog httpmodels.HTTPOpenSanctionCatalogResponse

	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return httpmodels.HTTPOpenSanctionCatalogDataset{}, err
	}

	for _, ds := range catalog.Datasets {
		if ds.Name == "lexisnexis" {
			return ds, nil
		}
	}

	return httpmodels.HTTPOpenSanctionCatalogDataset{}, errors.New("could not find a dataset named `lexisnexis` in the catalog")
}
