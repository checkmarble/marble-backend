package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/checkmarble/marble-backend/models"
)

func (db *Database) UserByEmail(ctx context.Context, email string) (models.User, error) {
	query := `
		SELECT id, email, first_name, last_name, role, organization_id, partner_id
		FROM users
		WHERE email = $1
		AND deleted_at IS NULL
	`

	var user models.User
	var organizationID *string
	var firstName, lastName pgtype.Text
	err := db.pool.QueryRow(ctx, query, email).
		Scan(&user.UserId,
			&user.Email,
			&firstName,
			&lastName,
			&user.Role,
			&organizationID,
			&user.PartnerId,
		)
	if firstName.Valid {
		user.FirstName = firstName.String
	}
	if lastName.Valid {
		user.LastName = lastName.String
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, models.NotFoundError
	}
	if err != nil {
		return models.User{}, fmt.Errorf("row.Scan error: %w", err)
	}
	if organizationID != nil {
		user.OrganizationId = *organizationID
	}
	return user, nil
}
