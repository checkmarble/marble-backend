package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (db *Database) UserByEmail(ctx context.Context, email string) (models.User, error) {
	query := `
		SELECT u.id, u.email, u.first_name, u.last_name, u.role, u.organization_id, o.tenant_id
		FROM users u
		LEFT JOIN organizations o ON o.id = u.organization_id
		WHERE u.email = $1
		AND u.deleted_at IS NULL
	`

	var user models.User
	var organizationID *string
	var tenantID *uuid.UUID
	var firstName, lastName pgtype.Text
	err := db.pool.QueryRow(ctx, query, email).
		Scan(&user.UserId,
			&user.Email,
			&firstName,
			&lastName,
			&user.Role,
			&organizationID,
			&tenantID,
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
		orgId, err := uuid.Parse(*organizationID)
		if err != nil {
			return models.User{}, fmt.Errorf("uuid.Parse error: %w", err)
		}
		user.OrganizationId = orgId
	}
	if tenantID != nil {
		user.TenantId = *tenantID
	}
	return user, nil
}

func (db *Database) ActiveGrantsForPrincipal(ctx context.Context, principalType, principalID string) ([]models.Grant, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT g.role, g.tenant_id, g.organization_id
		FROM active_grants g
		WHERE g.principal_type = $1 AND g.principal_id = $2 AND g.principal_authority = 'marble'
	`, principalType, principalID)
	if err != nil {
		return nil, fmt.Errorf("querying active grants: %w", err)
	}
	defer rows.Close()

	grants := []models.Grant{}
	for rows.Next() {
		var grant models.Grant
		var role string
		var tenantID, organizationID *uuid.UUID
		if err := rows.Scan(&role, &tenantID, &organizationID); err != nil {
			return nil, fmt.Errorf("scanning active grant: %w", err)
		}
		grant.Role = models.RoleFromString(role)
		if tenantID != nil {
			grant.TenantId = *tenantID
		}
		if organizationID != nil {
			grant.OrganizationId = *organizationID
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating active grants: %w", err)
	}
	return grants, nil
}

func (db *Database) UpdateUserProfileFromClaims(
	ctx context.Context,
	user models.User,
	profile models.IdentityUpdatableClaims,
) (models.User, error) {
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_USERS).
		Where("id = ?", user.UserId).
		Where(squirrel.Or{
			squirrel.NotEq{"picture": profile.Picture},
			squirrel.NotEq{"first_name": profile.Firstname},
			squirrel.NotEq{"last_name": profile.Lastname},
		})
	updated := false

	if profile.Firstname != "" && profile.Lastname != "" {
		query = query.Set("first_name", profile.Firstname).Set("last_name", profile.Lastname)
		updated = true
	}
	if profile.Picture != "" {
		query = query.Set("picture", profile.Picture)
		updated = true
	}

	if !updated {
		return user, nil
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return user, err
	}
	tag, err := db.pool.Exec(ctx, sql, args...)
	if err != nil {
		return user, err
	}
	if tag.RowsAffected() == 0 {
		return user, nil
	}

	return db.UserByEmail(ctx, user.Email)
}
