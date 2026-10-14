package auth

import (
	"net"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

type CredentialsType int

const (
	CredentialsBearer CredentialsType = iota
	CredentialsApiKey
)

type Extractor interface {
	Extract(r *http.Request) (Credentials, error)
}

type Credentials struct {
	Type     CredentialsType
	Value    string
	Fallback string
	// ClientIp is the address the request came from, if known.
	ClientIp net.IP
}

type MarbleExtractor struct{}

func DefaultExtractor() Extractor {
	return MarbleExtractor{}
}

func (e MarbleExtractor) Extract(r *http.Request) (Credentials, error) {
	if header := r.Header.Get("x-api-key"); header != "" {
		return Credentials{Type: CredentialsApiKey, Value: header}, nil
	}

	if header := r.Header.Get("authorization"); header != "" && strings.HasPrefix(header, "Bearer ") {
		return Credentials{
			Type:     CredentialsBearer,
			Value:    strings.TrimPrefix(header, "Bearer "),
			Fallback: r.Header.Get("x-oidc-access-token"),
		}, nil
	}

	return Credentials{}, errors.New("missing credentials in headers")
}
