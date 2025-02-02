package repositories

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/h2non/gock"
	"github.com/stretchr/testify/assert"
)

func getMockedOpenSanctionsRepository(host, authMethod, apiKey string) OpenSanctionsRepository {
	client := &http.Client{Transport: &http.Transport{}}

	gock.InterceptClient(client)

	return OpenSanctionsRepository{
		opensanctions: infra.InitializeOpenSanctions(client, host, authMethod, apiKey),
	}
}

func TestOpenSanctionsSelfHostedApi(t *testing.T) {
	defer gock.Off()

	repo := getMockedOpenSanctionsRepository("https://yente.local", "", "")
	cfg := models.SanctionCheckConfig{}
	query := models.OpenSanctionsQuery{
		Config: cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"bob"},
		},
		OrgConfig: models.OrganizationOpenSanctionsConfig{},
	}

	gock.New("https://yente.local").
		Post("/match/sanctions").
		Reply(http.StatusBadRequest)

	_, err := repo.Search(context.TODO(), query)

	assert.False(t, gock.HasUnmatchedRequest())
	assert.Error(t, err)
}

func TestOpenSanctionsSelfHostedAndApiKey(t *testing.T) {
	defer gock.Off()

	repo := getMockedOpenSanctionsRepository("https://yente.local", "", "abcdef")
	cfg := models.SanctionCheckConfig{}
	query := models.OpenSanctionsQuery{
		Config: cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"bob"},
		},
		OrgConfig: models.OrganizationOpenSanctionsConfig{},
	}

	gock.New("https://yente.local").
		Post("/match/sanctions").
		MatchParam("api_key", "abcdef").
		Reply(http.StatusBadRequest)

	_, err := repo.Search(context.TODO(), query)

	assert.False(t, gock.HasUnmatchedRequest())
	assert.Error(t, err)
}

func TestOpenSanctionsSaaSAndApiKey(t *testing.T) {
	defer gock.Off()

	repo := getMockedOpenSanctionsRepository("", "", "abcdef")
	cfg := models.SanctionCheckConfig{}
	query := models.OpenSanctionsQuery{
		Config: cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"bob"},
		},
		OrgConfig: models.OrganizationOpenSanctionsConfig{},
	}

	gock.New(infra.OPEN_SANCTIONS_API_HOST).
		Post("/match/sanctions").
		MatchParam("api_key", "abcdef").
		Reply(http.StatusBadRequest)

	_, err := repo.Search(context.TODO(), query)

	assert.False(t, gock.HasUnmatchedRequest())
	assert.Error(t, err)
}

func TestOpenSanctionsSelfHostedAndBearerToken(t *testing.T) {
	defer gock.Off()

	repo := getMockedOpenSanctionsRepository("https://yente.local", "bearer", "abcdef")
	cfg := models.SanctionCheckConfig{}
	query := models.OpenSanctionsQuery{
		Config: cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"bob"},
		},
		OrgConfig: models.OrganizationOpenSanctionsConfig{},
	}

	gock.New("https://yente.local").
		Post("/match/sanctions").
		MatchHeader("authorization", "Bearer abcdef").
		Reply(http.StatusBadRequest)

	_, err := repo.Search(context.TODO(), query)

	assert.False(t, gock.HasUnmatchedRequest())
	assert.Error(t, err)
}

func TestOpenSanctionsSelfHostedAndBasicAuth(t *testing.T) {
	defer gock.Off()

	repo := getMockedOpenSanctionsRepository("https://yente.local", "basic", "abcdef:helloworld")
	cfg := models.SanctionCheckConfig{}
	query := models.OpenSanctionsQuery{
		Config: cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"bob"},
		},
		OrgConfig: models.OrganizationOpenSanctionsConfig{},
	}

	gock.New("https://yente.local").
		Post("/match/sanctions").
		MatchHeader("authorization", "Basic YWJjZGVmOmhlbGxvd29ybGQ=").
		Reply(http.StatusBadRequest)

	_, err := repo.Search(context.TODO(), query)

	assert.False(t, gock.HasUnmatchedRequest())
	assert.Error(t, err)
}

func TestOpenSanctionsError(t *testing.T) {
	defer gock.Off()

	repo := getMockedOpenSanctionsRepository("", "", "")
	cfg := models.SanctionCheckConfig{}
	query := models.OpenSanctionsQuery{
		Config: cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"bob"},
		},
		OrgConfig: models.OrganizationOpenSanctionsConfig{},
	}

	gock.New(infra.OPEN_SANCTIONS_API_HOST).
		Post("/match/sanctions").
		Reply(http.StatusBadRequest)

	_, err := repo.Search(context.TODO(), query)

	assert.False(t, gock.HasUnmatchedRequest())
	assert.Error(t, err)
}

func TestOpenSanctionsSuccessfulPartialResponse(t *testing.T) {
	defer gock.Off()

	repo := getMockedOpenSanctionsRepository("", "", "")
	cfg := models.SanctionCheckConfig{}
	query := models.OpenSanctionsQuery{
		Config: cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"bob"},
		},
		OrgConfig: models.OrganizationOpenSanctionsConfig{},
	}

	body, _ := os.ReadFile("./fixtures/opensanctions/response_partial.json")

	gock.New(infra.OPEN_SANCTIONS_API_HOST).
		Post("/match/sanctions").
		Reply(http.StatusOK).
		BodyString(string(body))

	matches, err := repo.Search(context.TODO(), query)

	assert.False(t, gock.HasUnmatchedRequest())
	assert.NoError(t, err)
	assert.Len(t, matches.Matches, 1)
	assert.Equal(t, true, matches.Partial)
	assert.Contains(t, string(matches.Matches[0].Payload), "Joe")
}

func TestOpenSanctionsSuccessfulFullResponse(t *testing.T) {
	defer gock.Off()

	repo := getMockedOpenSanctionsRepository("", "", "")
	cfg := models.SanctionCheckConfig{}
	query := models.OpenSanctionsQuery{
		Config: cfg,
		Queries: models.OpenSanctionCheckFilter{
			"name": []string{"bob"},
		},
		OrgConfig: models.OrganizationOpenSanctionsConfig{},
	}

	body, _ := os.ReadFile("./fixtures/opensanctions/response_full.json")

	gock.New(infra.OPEN_SANCTIONS_API_HOST).
		Post("/match/sanctions").
		Reply(http.StatusOK).
		BodyString(string(body))

	matches, err := repo.Search(context.TODO(), query)

	assert.False(t, gock.HasUnmatchedRequest())
	assert.NoError(t, err)
	assert.Len(t, matches.Matches, 2)
	assert.Equal(t, false, matches.Partial)

	for idx := range 2 {
		if !strings.Contains(string(matches.Matches[idx].Payload), "Joe") &&
			!strings.Contains(string(matches.Matches[idx].Payload), "ACME Inc.") {
			t.Error("payloads did not contain required text")
		}
	}
}

func TestDatasetOutdatedDetector(t *testing.T) {
	type spec struct {
		schedule        string
		upstreamVersion string
		lastChange      time.Time
		localVersion    string
		updatedAt       time.Time
		expected        bool
	}

	now := func() time.Time {
		return time.Date(2025, 1, 23, 11, 0, 0, 0, time.UTC)
	}

	hr := func(offset int) time.Time {
		return now().Add(time.Duration(offset) * time.Hour)
	}

	tts := []spec{
		{"", "v1", hr(0), "v1", hr(-1000), true},
		{"* * * * *", "v2", hr(0), "v1", hr(-1000), false},
		{"0 */2 * * *", "v2", hr(0), "v1", hr(-1), true},
		{"0 */2 * * *", "v2", hr(-6), "v1", hr(-7), false},
		{"0 */12 * * *", "v2", hr(-6), "v1", hr(-7), true},
		{"0 */12 * * *", "v2", hr(-6), "v1", hr(-20), false},
		{"0 */12 * * *", "v2", hr(0), "v1", hr(-25), false},
	}

	for _, tt := range tts {
		dataset := models.OpenSanctionsDataset{
			Version:    tt.localVersion,
			LastExport: tt.updatedAt,
			Upstream: models.OpenSanctionsUpstreamDataset{
				Version:    tt.upstreamVersion,
				Schedule:   tt.schedule,
				LastExport: tt.lastChange,
			},
		}

		assert.NoError(t, dataset.CheckIsUpToDate(now))
		assert.Equal(t, tt.expected, dataset.UpToDate)
	}
}
