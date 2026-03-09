package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/singleflight"
)

const (
	OPEN_SANCTIONS_API_HOST = "https://api.opensanctions.org"
)

type OpenSanctionsAuthMethod int

const (
	OPEN_SANCTIONS_AUTH_SAAS OpenSanctionsAuthMethod = iota
	OPEN_SANCTIONS_AUTH_BEARER
	OPEN_SANCTIONS_AUTH_BASIC
)

type OpenSanctions struct {
	client      *http.Client
	host        string
	authMethod  OpenSanctionsAuthMethod
	credentials string
	algorithm   string
	scope       string

	motivaFeatureSemaphore *singleflight.Group
	motivaFeatures         *atomic.Pointer[MotivaFeatures]

	nameRecognition *NameRecognitionProvider
}

type NameRecognitionProvider struct {
	ApiUrl string
	ApiKey string
}

type MotivaFeatures struct {
	BodyParams  bool
	ScopedIndex bool
}

func InitializeOpenSanctions(ctx context.Context, client *http.Client, host, authMethod, creds string) OpenSanctions {
	os := OpenSanctions{
		client:                 client,
		host:                   host,
		credentials:            creds,
		algorithm:              "logic-v1",
		scope:                  "default",
		motivaFeatures:         &atomic.Pointer[MotivaFeatures]{},
		motivaFeatureSemaphore: &singleflight.Group{},
	}

	os.MotivaFeatures(ctx)

	if os.IsSelfHosted() {
		switch authMethod {
		case "bearer":
			os.authMethod = OPEN_SANCTIONS_AUTH_BEARER
		case "basic":
			os.authMethod = OPEN_SANCTIONS_AUTH_BASIC
		}
	}

	return os
}

func (os *OpenSanctions) WithAlgorithm(algo string) *OpenSanctions {
	os.algorithm = algo

	return os
}

func (os *OpenSanctions) WithScope(scope string) *OpenSanctions {
	os.scope = scope

	return os
}

func (os *OpenSanctions) WithNameRecognition(apiUrl, apiKey string) *OpenSanctions {
	os.nameRecognition = &NameRecognitionProvider{
		ApiUrl: apiUrl,
		ApiKey: apiKey,
	}

	return os
}

func (os OpenSanctions) Client() *http.Client {
	return os.client
}

func (os OpenSanctions) IsConfigured() (bool, error) {
	if !os.IsSelfHosted() && len(os.credentials) == 0 {
		return false, fmt.Errorf("missing API key for SaaS Open Sanctions configuration")
	}
	return true, nil
}

func (os OpenSanctions) IsSelfHosted() bool {
	return len(os.host) > 0
}

func (os OpenSanctions) Host() string {
	if os.IsSelfHosted() {
		return os.host
	}

	return OPEN_SANCTIONS_API_HOST
}

func (os OpenSanctions) IsSet() bool {
	return os.IsSelfHosted() || os.Credentials() != ""
}

func (os OpenSanctions) AuthMethod() OpenSanctionsAuthMethod {
	return os.authMethod
}

func (os OpenSanctions) Credentials() string {
	return os.credentials
}

func (os OpenSanctions) NameRecognition() *NameRecognitionProvider {
	return os.nameRecognition
}

func (os OpenSanctions) Scope() string {
	return os.scope
}

func (os OpenSanctions) Algorithm() string {
	return os.algorithm
}

func (os OpenSanctions) IsNameRecognitionSet() bool {
	return os.nameRecognition != nil && os.nameRecognition.ApiUrl != ""
}

func (os *OpenSanctions) MotivaFeatures(ctx context.Context) MotivaFeatures {
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

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/-/version", os.Host()), nil)
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
