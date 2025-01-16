package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/httpmodels"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

const (
	// TODO: Pull this as server configuration
	DEV_YENTE_URL = "http://app.yente.orb.local"
)

type OpenSanctionsRepository struct{}

type openSanctionsRequest struct {
	Queries map[string]openSanctionsRequestQuery `json:"queries"`
}

type openSanctionsRequestQuery struct {
	Schema     string                         `json:"schema"`
	Properties models.OpenSanctionCheckFilter `json:"properties"`
}

func (repo OpenSanctionsRepository) Search(ctx context.Context, cfg models.SanctionCheckConfig,
	query models.OpenSanctionsQuery,
) (models.SanctionCheckResult, error) {
	req, err := repo.searchRequest(ctx, query)
	if err != nil {
		return models.SanctionCheckResult{}, err
	}

	utils.LoggerFromContext(ctx).Debug("SANCTION CHECK: sending request...")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.SanctionCheckResult{}, errors.Wrap(err, "could not perform sanction check")
	}

	if resp.StatusCode != http.StatusOK {
		return models.SanctionCheckResult{}, fmt.Errorf(
			"sanction check API returned status %d", resp.StatusCode)
	}

	var matches httpmodels.HTTPOpenSanctionsResult

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&matches); err != nil {
		return models.SanctionCheckResult{}, errors.Wrap(err,
			"could not parse sanction check response")
	}

	return httpmodels.AdaptOpenSanctionsResult(matches)
}

func (OpenSanctionsRepository) searchRequest(ctx context.Context, query models.OpenSanctionsQuery) (*http.Request, error) {
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
		return nil, errors.Wrap(err, "could not parse OpenSanctions response")
	}

	url := fmt.Sprintf("%s/match/sanctions", DEV_YENTE_URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)

	return req, err
}
