package auth

import (
	"net/http"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
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
	TenantId uuid.UUID
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
		credentials := Credentials{Type: CredentialsBearer, Value: strings.TrimPrefix(header, "Bearer "), Fallback: r.Header.Get("x-oidc-access-token")}
		if tenantId := r.Header.Get("x-marble-tenant-id"); tenantId != "" {
			parsedTenantId, err := uuid.Parse(tenantId)
			if err != nil {
				return Credentials{}, errors.New("invalid tenant ID")
			}
			credentials.TenantId = parsedTenantId
		}
		return credentials, nil
	}

	return Credentials{}, errors.New("missing credentials in headers")
}
