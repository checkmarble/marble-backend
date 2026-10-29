package postgres

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
)

// ResolveUserIdentity returns the user bound to an identity. On its first login,
// an accepted email links that identity to the existing user.
func (db *Database) ResolveUserIdentity(ctx context.Context, issuer, subject, email string) (models.User, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return models.User{}, errors.Wrap(err, "beginning user identity resolution")
	}
	defer tx.Rollback(ctx)

	var userID string
	resolution := "identity"
	query, args, err := NewQueryBuilder().Select("u.id").
		From("user_identities i").
		Join("users u ON u.id = i.user_id").
		Where(squirrel.Eq{"i.issuer": issuer, "i.subject": subject, "u.deleted_at": nil}).
		ToSql()
	if err != nil {
		return models.User{}, errors.Wrap(err, "building user identity query")
	}
	err = tx.QueryRow(ctx, query, args...).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		resolution = "email_link"
		query, args, err = NewQueryBuilder().Select("id").
			From(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"email": email, "deleted_at": nil}).
			ToSql()
		if err != nil {
			return models.User{}, errors.Wrap(err, "building user email query")
		}
		err = tx.QueryRow(ctx, query, args...).Scan(&userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.User{}, models.NotFoundError
			}
			return models.User{}, errors.Wrap(err, "finding user by email")
		}
		query, args, err = NewQueryBuilder().Insert("user_identities").
			Columns("user_id", "issuer", "subject").
			Values(userID, issuer, subject).
			Suffix("ON CONFLICT DO NOTHING").
			ToSql()
		if err != nil {
			return models.User{}, errors.Wrap(err, "building user identity insert")
		}
		tag, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return models.User{}, errors.Wrap(err, "linking user identity")
		}
		if tag.RowsAffected() == 0 {
			resolution = "identity"
		}
		query, args, err = NewQueryBuilder().Select("user_id").
			From("user_identities").
			Where(squirrel.Eq{"issuer": issuer, "subject": subject}).
			ToSql()
		if err != nil {
			return models.User{}, errors.Wrap(err, "building linked user identity query")
		}
		if err := tx.QueryRow(ctx, query, args...).Scan(&userID); err != nil {
			return models.User{}, errors.Wrap(err, "reading linked user identity")
		}
	} else if err != nil {
		return models.User{}, errors.Wrap(err, "finding user identity")
	}

	user, err := userByID(ctx, tx, userID)
	if err != nil {
		return models.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.User{}, errors.Wrap(err, "committing user identity resolution")
	}
	utils.LoggerFromContext(ctx).InfoContext(ctx, "identity resolved",
		"resolution", resolution,
		"issuer", issuer,
		"user_id", user.UserId,
	)
	return user, nil
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
		return user, errors.Wrap(err, "building user profile update")
	}
	tag, err := db.pool.Exec(ctx, sql, args...)
	if err != nil {
		return user, errors.Wrap(err, "updating user profile")
	}
	if tag.RowsAffected() == 0 {
		return user, nil
	}

	return userByID(ctx, db.pool, string(user.UserId))
}

type userGetter interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func userByID(ctx context.Context, db userGetter, userID string) (models.User, error) {
	query, args, err := NewQueryBuilder().
		Select("u.id", "u.email", "u.first_name", "u.last_name", "u.role", "u.organization_id", "o.tenant_id").
		From("users u").
		LeftJoin("organizations o ON o.id = u.organization_id").
		Where(squirrel.Eq{"u.id": userID, "u.deleted_at": nil}).
		ToSql()
	if err != nil {
		return models.User{}, errors.Wrap(err, "building user lookup")
	}

	var user models.User
	var organizationID *string
	var tenantID *uuid.UUID
	var firstName, lastName pgtype.Text
	err = db.QueryRow(ctx, query, args...).Scan(&user.UserId, &user.Email, &firstName, &lastName, &user.Role, &organizationID, &tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, models.NotFoundError
	}
	if err != nil {
		return models.User{}, errors.Wrap(err, "scanning user")
	}
	if firstName.Valid {
		user.FirstName = firstName.String
	}
	if lastName.Valid {
		user.LastName = lastName.String
	}
	if organizationID != nil {
		organizationID, err := uuid.Parse(*organizationID)
		if err != nil {
			return models.User{}, errors.Wrap(err, "parsing organization ID")
		}
		user.OrganizationId = organizationID
	}
	if tenantID != nil {
		user.TenantId = *tenantID
	}
	return user, nil
}
