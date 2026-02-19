package infra

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
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

	isMotivaSemaphore *singleflight.Group
	isMotiva          *atomic.Pointer[bool]

	nameRecognition *NameRecognitionProvider
}

type NameRecognitionProvider struct {
	ApiUrl string
	ApiKey string
}

func InitializeOpenSanctions(ctx context.Context, client *http.Client, host, authMethod, creds string) OpenSanctions {
	os := OpenSanctions{
		client:            client,
		host:              host,
		credentials:       creds,
		algorithm:         "logic-v1",
		scope:             "default",
		isMotiva:          &atomic.Pointer[bool]{},
		isMotivaSemaphore: &singleflight.Group{},
	}

	os.IsMotiva(ctx)

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

func (os *OpenSanctions) IsMotiva(ctx context.Context) bool {
	logger := utils.LoggerFromContext(ctx)

	if isMotiva := os.isMotiva.Load(); isMotiva != nil {
		return *isMotiva
	}

	// If the initial fingerprint fails, we will need to do it during querying,
	// that could potentially be a lot of requests, so we singleflight them.
	isMotiva, err, _ := os.isMotivaSemaphore.Do("motiva", func() (any, error) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, fmt.Sprintf("%s/-/version", os.Host()), nil)
		if err != nil {
			return false, err
		}

		resp, err := http.DefaultClient.Do(req)

		if err == nil {
			isMotiva := resp.StatusCode == http.StatusOK

			os.isMotiva.Store(&isMotiva)

			return isMotiva, nil
		}

		return false, errors.Wrapf(err, "could not determine whether motiva is used")
	})
	if err != nil {
		logger.Warn(err.Error())

		return false
	}

	return isMotiva.(bool)
}
