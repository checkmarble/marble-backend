package dbmodels

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestAdaptOrganizationUserGrant(t *testing.T) {
	tests := []struct {
		name string
		role *string
		want *models.Role
	}{
		{
			name: "with organization grant role",
			role: stringPointer("ADMIN"),
			want: rolePointer(models.ADMIN),
		},
		{
			name: "without organization grant role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AdaptOrganizationUserGrant(DBOrganizationUserGrant{
				DBUserResult: DBUserResult{
					Id:              "user-id",
					Email:           "user@example.com",
					Role:            int(models.ADMIN),
					FirstName:       pgtype.Text{String: "Test", Valid: true},
					LastName:        pgtype.Text{String: "User", Valid: true},
					AiAssistEnabled: true,
				},
				OrganizationGrantRole: tt.role,
			})

			require.NoError(t, err)
			require.Equal(t, models.UserId("user-id"), got.User.UserId)
			require.Equal(t, "user@example.com", got.User.Email)
			require.Equal(t, tt.want, got.OrganizationGrantRole)
		})
	}
}

func stringPointer(value string) *string { return &value }

func rolePointer(value models.Role) *models.Role { return &value }
