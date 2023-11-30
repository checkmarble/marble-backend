package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/checkmarble/marble-backend/models"
)

func (db *Database) UserByFirebaseUid(ctx context.Context, firebaseUID string) (models.User, error) {
	query := `
		SELECT id, email, first_name, last_name, firebase_uid, role, organization_id
		FROM users
		WHERE firebase_uid = $1
	`

	var user models.User
	var organizationID *string
	var firstName, lastName pgtype.Text
	err := db.pool.QueryRow(ctx, query, firebaseUID).
		Scan(&user.UserId,
			&user.Email,
			&firstName,
			&lastName,
			&user.FirebaseUid,
			&user.Role,
			&organizationID,
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

func (db *Database) UserByEmail(ctx context.Context, email string) (models.User, error) {
	query := `
		SELECT id, email, first_name, last_name, firebase_uid, role, organization_id
		FROM users
		WHERE email = $1
	`

	var user models.User
	var organizationID *string
	var firstName, lastName pgtype.Text
	err := db.pool.QueryRow(ctx, query, email).
		Scan(&user.UserId,
			&user.Email,
			&firstName,
			&lastName,
			&user.FirebaseUid,
			&user.Role,
			&organizationID,
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

func (db *Database) UpdateUserFirebaseUID(ctx context.Context, userID models.UserId, firebaseUID string) error {
	query := `
		UPDATE users
		SET firebase_uid = $2
		WHERE id = $1
	`

	_, err := db.pool.Exec(ctx, query, userID, firebaseUID)
	if err != nil {
		return fmt.Errorf("pool.Exec error: %w", err)
	}
	return nil
}
