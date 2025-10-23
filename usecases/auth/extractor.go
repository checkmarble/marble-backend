package auth

import (
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
}

type MarbleExtractor struct{}

func DefaultExtractor() Extractor {
	return MarbleExtractor{}
}

func (e MarbleExtractor) Extract(r *http.Request) (Credentials, error) {
	if header := r.Header.Get("x-api-key"); header != "" {
		return Credentials{CredentialsApiKey, header, ""}, nil
	}

	if header := r.Header.Get("authorization"); header != "" && strings.HasPrefix(header, "Bearer ") {
		return Credentials{CredentialsBearer, strings.TrimPrefix(header, "Bearer "), r.Header.Get("x-oidc-access-token")}, nil
	}

	return Credentials{}, errors.New("missing credentials in headers")
}
