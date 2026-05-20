package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/singleflight"
)

const (
	OPEN_SANCTIONS_API_HOST = "https://api.opensanctions.org"
)

type ScreeningAuthMethod int

const (
	SCREENING_AUTH_SAAS ScreeningAuthMethod = iota
	SCREENING_AUTH_BEARER
	SCREENING_AUTH_BASIC
)

type Screening struct {
	client *http.Client

	providers map[models.ScreeningProvider]*ScreeningProvider

	authMethod  ScreeningAuthMethod
	credentials string
	algorithm   string

	motivaFeatureSemaphore *singleflight.Group
	motivaFeatures         *atomic.Pointer[MotivaFeatures]

	nameRecognition *NameRecognitionProvider
}

type ScreeningProvider struct {
	host  string
	scope string
}

type NameRecognitionProvider struct {
	ApiUrl string
	ApiKey string
}

type MotivaFeatures struct {
	BodyParams  bool
	ScopedIndex bool
}

func InitializeScreening(ctx context.Context, client *http.Client, host, authMethod, creds string) Screening {
	os := Screening{
		client:                 client,
		providers:              map[models.ScreeningProvider]*ScreeningProvider{},
		credentials:            creds,
		algorithm:              "logic-v1",
		motivaFeatures:         &atomic.Pointer[MotivaFeatures]{},
		motivaFeatureSemaphore: &singleflight.Group{},
	}

	if host != "" {
		os.providers[models.ScreeningProviderOpenSanctions] = &ScreeningProvider{
			host:  host,
			scope: "default",
		}
	}

	os.MotivaFeatures(ctx)

	if os.IsSelfHosted(models.ScreeningProviderOpenSanctions) {
		switch authMethod {
		case "bearer":
			os.authMethod = SCREENING_AUTH_BEARER
		case "basic":
			os.authMethod = SCREENING_AUTH_BASIC
		}
	}

	return os
}

func (os *Screening) WithAlgorithm(algo string) *Screening {
	os.algorithm = algo

	return os
}

func (os *Screening) WithScope(scope string) *Screening {
	os.providers[models.ScreeningProviderOpenSanctions].scope = scope

	return os
}

func (os *Screening) WithLexisNexisHost(host string) *Screening {
	os.providers["lexisnexis"] = &ScreeningProvider{
		host:  host,
		scope: "lexisnexis",
	}

	return os
}

func (os *Screening) WithNameRecognition(apiUrl, apiKey string) *Screening {
	os.nameRecognition = &NameRecognitionProvider{
		ApiUrl: apiUrl,
		ApiKey: apiKey,
	}

	return os
}

func (os Screening) Client() *http.Client {
	return os.client
}

func (os Screening) IsConfigured(provider models.ScreeningProvider) (bool, error) {
	if !os.IsSelfHosted(provider) && len(os.credentials) == 0 {
		return false, fmt.Errorf("missing API key for SaaS Open Sanctions configuration")
	}
	return true, nil
}

func (os Screening) IsSelfHosted(provider models.ScreeningProvider) bool {
	if p, ok := os.providers[provider]; ok {
		return len(p.host) > 0
	}
	return false
}

func (os Screening) Host(provider models.ScreeningProvider) string {
	if os.IsSelfHosted(provider) {
		if p, ok := os.providers[provider]; ok {
			return p.host
		}
		return ""
	}

	return OPEN_SANCTIONS_API_HOST
}

func (os Screening) IsSet() bool {
	return os.IsSelfHosted(models.ScreeningProviderOpenSanctions) || os.Credentials() != ""
}

func (os Screening) AuthMethod() ScreeningAuthMethod {
	return os.authMethod
}

func (os Screening) Credentials() string {
	return os.credentials
}

func (os Screening) NameRecognition() *NameRecognitionProvider {
	return os.nameRecognition
}

func (os Screening) Scope(provider models.ScreeningProvider) string {
	if p, ok := os.providers[provider]; ok {
		return p.scope
	}
	return "default"
}

func (os Screening) Algorithm() string {
	return os.algorithm
}

func (os Screening) IsNameRecognitionSet() bool {
	return os.nameRecognition != nil && os.nameRecognition.ApiUrl != ""
}

func (os *Screening) MotivaFeatures(ctx context.Context) MotivaFeatures {
	logger := utils.LoggerFromContext(ctx)

	if isMotiva := os.motivaFeatures.Load(); isMotiva != nil {
		return *isMotiva
	}

	type motivaVersionInfo struct {
		Motiva string `json:"motiva"`
	}

	// If the initial fingerprint fails, we will need to do it during querying,
	// that could potentially be a lot of requests, so we singleflight them.
	motivaFeatures, err, _ := os.motivaFeatureSemaphore.Do("motiva", func() (any, error) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/-/version", os.Host(models.ScreeningProviderOpenSanctions)), nil)
		if err != nil {
			return false, err
		}

		resp, err := http.DefaultClient.Do(req)

		if err == nil {
			defer resp.Body.Close()

			feats := MotivaFeatures{}

			if resp.StatusCode == http.StatusOK {
				var v motivaVersionInfo

				if err := json.NewDecoder(resp.Body).Decode(&v); err == nil {
					// Features introduced in motiva v0.7.0
					//  - transmit unbounded query parameters in request body
					//  - use scoped index
					feats.BodyParams = semver.Compare(v.Motiva, "v0.7.0") >= 0
					feats.ScopedIndex = semver.Compare(v.Motiva, "v0.7.0") >= 0
				}
			}

			os.motivaFeatures.Store(&feats)

			return feats, nil
		}

		return false, errors.Wrapf(err, "could not determine whether motiva is used")
	})
	if err != nil {
		logger.Warn(err.Error())

		return MotivaFeatures{}
	}

	return motivaFeatures.(MotivaFeatures)
}
