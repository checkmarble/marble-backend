package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMarbleExtractorExtractsTenantHint(t *testing.T) {
	tenantId := uuid.New()
	request := httptest.NewRequest("GET", "/token", nil)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("X-Marble-Tenant-ID", tenantId.String())

	credentials, err := DefaultExtractor().Extract(request)

	require.NoError(t, err)
	require.Equal(t, tenantId, credentials.TenantId)
}

func TestMarbleExtractorRejectsInvalidTenantHint(t *testing.T) {
	request := httptest.NewRequest("GET", "/token", nil)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("X-Marble-Tenant-ID", "not-a-uuid")

	_, err := DefaultExtractor().Extract(request)

	require.ErrorContains(t, err, "invalid tenant ID")
}
