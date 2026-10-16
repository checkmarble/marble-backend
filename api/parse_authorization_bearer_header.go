package api

import (
	"net/http"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
)

func ParseAuthorizationBearerHeader(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	if authorization == "" {
		return "", nil
	}

	authHeader := strings.Split(header.Get("Authorization"), "Bearer ")
	if len(authHeader) != 2 {
		return "", errors.Errorf("malformed token: %w", models.UnAuthorizedError)
	}
	return authHeader[1], nil
}
