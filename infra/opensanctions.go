package infra

import (
	"fmt"
	"net/http"
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

	nameRecognition *NameRecognitionProvider
}

type NameRecognitionProvider struct {
	ApiUrl string
	ApiKey string
}

func InitializeOpenSanctions(client *http.Client, host, authMethod, creds string) OpenSanctions {
	os := OpenSanctions{
		client:      client,
		host:        host,
		credentials: creds,
	}

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
