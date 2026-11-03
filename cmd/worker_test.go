package cmd

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSingleJobContext(t *testing.T) {
	orgID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	creds := models.Credentials{Roles: []models.Role{models.SYSTEM}}
	for _, tc := range []struct {
		name, args string
		orgID      uuid.UUID
		wantError  bool
	}{
		{"organization", `{"org_id":"11111111-1111-4111-8111-111111111111"}`, orgID, false},
		{"empty arguments", "", uuid.Nil, false},
		{"no organization", `{"case_id":"22222222-2222-4222-8222-222222222222"}`, uuid.Nil, false},
		{"invalid organization", `{"org_id":"invalid"}`, uuid.Nil, true},
		{"invalid JSON", "{", uuid.Nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := singleJobContext(context.Background(), creds, tc.args)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, found := utils.CredentialsFromCtx(ctx)
			require.True(t, found)
			require.Equal(t, tc.orgID, got.OrganizationId)
			require.Equal(t, []models.Role{models.SYSTEM}, got.Roles)
		})
	}
}
