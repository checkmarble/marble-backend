package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/checkmarble/marble-backend/models"
)

func ParseAuthorizationBearerHeader(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	if authorization == "" {
		return "", nil
	}

	authHeader := strings.Split(header.Get("Authorization"), "Bearer ")
	if len(authHeader) != 2 {
		return "", fmt.Errorf("malformed token: %w", models.UnAuthorizedError)
	}

	return authHeader[1], nil
}
