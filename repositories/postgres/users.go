package postgres

import (
	"context"
	"strings"

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
	return db.resolveUserIdentity(ctx, issuer, subject, email, uuid.Nil)
}

// ResolveUserIdentityForTenant provisions tenant identities, never organization grants.
func (db *Database) ResolveUserIdentityForTenant(ctx context.Context, issuer, subject, email string, tenantID uuid.UUID) (models.User, error) {
	return db.resolveUserIdentity(ctx, issuer, subject, email, tenantID)
}

func (db *Database) resolveUserIdentity(ctx context.Context, issuer, subject, email string, tenantID uuid.UUID) (models.User, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return models.User{}, errors.Wrap(err, "beginning user identity resolution")
	}
	defer tx.Rollback(ctx)
	email = strings.ToLower(strings.TrimSpace(email))
	if tenantID != uuid.Nil {
		var lockedID uuid.UUID
		query, args, err := NewQueryBuilder().Select("id").From("tenants").
			Where(squirrel.Eq{"id": tenantID, "deleted_at": nil}).Suffix("FOR SHARE").ToSql()
		if err != nil {
			return models.User{}, err
		}
		if err := tx.QueryRow(ctx, query, args...).Scan(&lockedID); err != nil {
			return models.User{}, errors.Wrap(err, "locking authentication tenant")
		}
		if issuer == "" || subject == "" || email == "" {
			return models.User{}, models.BadParameterError
		}
	}
	// Serialize first logins for an email; the unique identity index arbitrates different emails for one subject.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", email); err != nil {
		return models.User{}, errors.Wrap(err, "locking identity email")
	}

	var userID string
	resolution := "identity"
	query := NewQueryBuilder().Select("u.id").
		From("user_identities i").
		Join("users u ON u.id = i.user_id").
		Where(squirrel.Eq{"i.issuer": issuer, "i.subject": subject})
	querySQL, args, err := query.ToSql()
	if err != nil {
		return models.User{}, errors.Wrap(err, "building user identity query")
	}
	err = tx.QueryRow(ctx, querySQL, args...).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		resolution = "email_link"
		query = NewQueryBuilder().Select("u.id").
			From(dbmodels.TABLE_USERS + " u").
			Where(squirrel.Eq{"u.email": email, "u.deleted_at": nil})
		if tenantID != uuid.Nil {
			query = query.Where(squirrel.Eq{"u.tenant_id": tenantID})
		}
		querySQL, args, err = query.ToSql()
		if err != nil {
			return models.User{}, errors.Wrap(err, "building user email query")
		}
		err = tx.QueryRow(ctx, querySQL, args...).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) && tenantID != uuid.Nil {
			var deleted bool
			checkSQL, checkArgs, buildErr := NewQueryBuilder().Select().Column("EXISTS (SELECT 1 FROM users WHERE lower(email) = ? AND deleted_at IS NOT NULL)", email).ToSql()
			if buildErr != nil {
				return models.User{}, buildErr
			}
			if checkErr := tx.QueryRow(ctx, checkSQL, checkArgs...).Scan(&deleted); checkErr != nil {
				return models.User{}, checkErr
			}
			if deleted {
				return models.User{}, models.NotFoundError
			}
			userID = uuid.NewString()
			if err := setIdentityAuditContext(ctx, tx.Tx, userID, tenantID); err != nil {
				return models.User{}, err
			}
			insertSQL, insertArgs, buildErr := NewQueryBuilder().Insert("users").
				Columns("id", "email", "role", "tenant_id").Values(userID, email, int(models.NO_ROLE), tenantID).
				Suffix("ON CONFLICT DO NOTHING RETURNING id").ToSql()
			if buildErr != nil {
				return models.User{}, buildErr
			}
			err = tx.QueryRow(ctx, insertSQL, insertArgs...).Scan(&userID)
		}
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.User{}, models.NotFoundError
			}
			return models.User{}, errors.Wrap(err, "finding user by email")
		}
		if err := setIdentityAuditContext(ctx, tx.Tx, userID, tenantID); err != nil {
			return models.User{}, err
		}
		identityQuery, args, err := NewQueryBuilder().Insert("user_identities").
			Columns("user_id", "issuer", "subject").
			Values(userID, issuer, subject).
			Suffix("ON CONFLICT DO NOTHING").
			ToSql()
		if err != nil {
			return models.User{}, errors.Wrap(err, "building user identity insert")
		}
		tag, err := tx.Exec(ctx, identityQuery, args...)
		if err != nil {
			return models.User{}, errors.Wrap(err, "linking user identity")
		}
		if tag.RowsAffected() == 0 {
			tx.Rollback(ctx)
			return db.resolveUserIdentity(ctx, issuer, subject, email, tenantID)
		}
		query = NewQueryBuilder().Select("u.id").
			From("user_identities").
			Join("users u ON u.id = user_identities.user_id").
			Where(squirrel.Eq{"user_identities.issuer": issuer, "user_identities.subject": subject, "u.deleted_at": nil})
		if tenantID != uuid.Nil {
			query = query.Where(squirrel.Eq{"u.tenant_id": tenantID})
		}
		querySQL, args, err = query.ToSql()
		if err != nil {
			return models.User{}, errors.Wrap(err, "building linked user identity query")
		}
		if err := tx.QueryRow(ctx, querySQL, args...).Scan(&userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.User{}, models.NotFoundError
			}
			return models.User{}, errors.Wrap(err, "reading linked user identity")
		}
	} else if err != nil {
		return models.User{}, errors.Wrap(err, "finding user identity")
	}

	user, err := userByID(ctx, tx, userID)
	if err != nil {
		return models.User{}, err
	}
	if tenantID != uuid.Nil && user.TenantId != tenantID {
		return models.User{}, models.NotFoundError
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
		Where("deleted_at IS NULL")
	changes := squirrel.Or{}
	if user.TenantId == uuid.Nil {
		query = query.Where(squirrel.Eq{"tenant_id": nil})
	} else {
		query = query.Where(squirrel.Eq{"tenant_id": user.TenantId})
	}

	if profile.Firstname != "" && profile.Lastname != "" {
		query = query.Set("first_name", profile.Firstname).Set("last_name", profile.Lastname)
		changes = append(changes, squirrel.Expr("first_name IS DISTINCT FROM ?", profile.Firstname), squirrel.Expr("last_name IS DISTINCT FROM ?", profile.Lastname))
	}
	if profile.Picture != "" {
		query = query.Set("picture", profile.Picture)
		changes = append(changes, squirrel.Expr("picture IS DISTINCT FROM ?", profile.Picture))
	}

	if len(changes) == 0 {
		return user, nil
	}

	sql, args, err := query.Where(changes).ToSql()
	if err != nil {
		return user, errors.Wrap(err, "building user profile update")
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return user, errors.Wrap(err, "beginning profile update")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := setIdentityAuditContext(ctx, tx, string(user.UserId), user.TenantId); err != nil {
		return user, err
	}
	_, err = tx.Exec(ctx, sql, args...)
	if err != nil {
		return user, errors.Wrap(err, "updating user profile")
	}
	result, err := userByID(ctx, tx, string(user.UserId))
	if err != nil {
		return user, err
	}
	if err := tx.Commit(ctx); err != nil {
		return user, err
	}
	return result, nil
}

func setIdentityAuditContext(ctx context.Context, tx pgx.Tx, userID string, tenantID uuid.UUID) error {
	tenant := ""
	if tenantID != uuid.Nil {
		tenant = tenantID.String()
	}
	// Transaction-local settings cannot inherit or leak another pooled request's actor.
	query, args, err := NewQueryBuilder().Select().
		Column("set_config('custom.current_user_id', ?, true)", userID).
		Column("set_config('custom.current_api_key_id', '', true)").
		Column("set_config('custom.current_org_id', '', true)").
		Column("set_config('custom.current_tenant_id', ?, true)", tenant).ToSql()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}

type userGetter interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (db *Database) GetUserByID(ctx context.Context, userID string) (models.User, error) {
	return userByID(ctx, db.pool, userID)
}

func userByID(ctx context.Context, db userGetter, userID string) (models.User, error) {
	query, args, err := NewQueryBuilder().
		Select("u.id", "u.email", "u.first_name", "u.last_name", "u.role", "u.tenant_id").
		From("users u").
		Where(squirrel.Eq{"u.id": userID, "u.deleted_at": nil}).
		ToSql()
	if err != nil {
		return models.User{}, errors.Wrap(err, "building user lookup")
	}

	var user models.User
	var tenantID *uuid.UUID
	var firstName, lastName pgtype.Text
	err = db.QueryRow(ctx, query, args...).Scan(&user.UserId, &user.Email, &firstName, &lastName, &user.Role, &tenantID)
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
	if tenantID != nil {
		user.TenantId = *tenantID
	}
	return user, nil
}
