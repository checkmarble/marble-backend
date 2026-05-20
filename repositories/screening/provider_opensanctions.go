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
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type ScreeningOpenSanctionsProvider struct {
	Config infra.Screening
}

func (p ScreeningOpenSanctionsProvider) SearchRequest(ctx context.Context,
	query *models.OpenSanctionsQuery,
) (*http.Request, []byte, error) {
	q := openSanctionsRequest{
		Queries: make(map[string]openSanctionsRequestQuery, len(query.Queries)),
	}

	if p.Config.MotivaFeatures(ctx).BodyParams {
		q.Params = &motivaRequestParams{}

		if len(query.Config.Datasets) > 0 {
			q.Params.IncludeDatasets = query.Config.Datasets
		}
		if len(query.WhitelistedEntityIds) > 0 {
			q.Params.ExcludeEntityIds = query.WhitelistedEntityIds
		}
	}

	for _, subquery := range query.Queries {
		q.Queries[pure_utils.NewId().String()] = openSanctionsRequestQuery{
			Schema:     subquery.Type,
			Properties: subquery.Filters,
		}
	}

	scope := p.Config.Scope(models.ScreeningProviderOpenSanctions)
	if query.Scope != "" {
		scope = query.Scope
	}

	var body, rawQuery bytes.Buffer

	if err := json.NewEncoder(io.MultiWriter(&body, &rawQuery)).Encode(q); err != nil {
		return nil, nil, errors.Wrap(err,
			"could not parse OpenSanctions response")
	}

	requestUrl := fmt.Sprintf("%s/match/%s", p.Config.Host(models.ScreeningProviderOpenSanctions), scope)

	if qs := p.BuildQueryString(ctx, &query.Config, query); len(qs) > 0 {
		requestUrl = fmt.Sprintf("%s?%s", requestUrl, qs.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestUrl, &body)
	req.Header.Set("content-type", "application/json")

	return req, rawQuery.Bytes(), err
}

func (p ScreeningOpenSanctionsProvider) BuildQueryString(ctx context.Context,
	cfg *models.ScreeningConfig, query *models.OpenSanctionsQuery,
) url.Values {
	qs := url.Values{}

	if p.Config.AuthMethod() == infra.SCREENING_AUTH_SAAS &&
		len(p.Config.Credentials()) > 0 {
		qs.Set("api_key", p.Config.Credentials())
	}

	if !p.Config.MotivaFeatures(ctx).BodyParams && cfg != nil && len(cfg.Datasets) > 0 {
		qs["include_dataset"] = cfg.Datasets
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

func (p ScreeningOpenSanctionsProvider) FindAvailableFilters(ctx context.Context) (dto.ScreeningAvailableFilters, error) {
	return dto.ScreeningAvailableFilters{}, nil
}
