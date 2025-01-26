package infra

import "net/http"

const (
	OPEN_SANCTIONS_API_HOST = "https://api.opensanctions.org"
)

type OpenSanctions struct {
	client *http.Client
	host   string
	// TODO: this is only for SaaS OpenSanctions API, we may need to abstract
	// over authentication to at least offer Basic and Bearer for self-hosted.
	apiKey string
}

func InitializeOpenSanctions(client *http.Client, host, apiKey string) OpenSanctions {
	return OpenSanctions{
		client: client,
		host:   host,
		apiKey: apiKey,
	}
}

func (os OpenSanctions) Client() *http.Client {
	return os.client
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

func (os OpenSanctions) ApiKey() string {
	return os.apiKey
}
