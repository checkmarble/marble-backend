package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/redis/go-redis/v9"

	"github.com/google/uuid"
)

type UserRepository interface {
	CreateUser(ctx context.Context, tx Transaction, createUser models.CreateUser) (string, error)
	UpdateUser(ctx context.Context, tx Transaction, updateUser models.UpdateUser) error
	DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error
	DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error
	UserById(ctx context.Context, exec Executor, userId string) (models.User, error)
	ListUsers(ctx context.Context, exec Executor, organizationId *uuid.UUID) ([]models.User, error)
	UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error)
	HasUsers(ctx context.Context, exec Executor) (bool, error)

	ListRoles(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.RbacRole, error)
	GetRoleBySlug(ctx context.Context, exec Executor, orgId uuid.UUID, slug models.Role) (models.RbacRole, error)
	GetRoleBySlugForUpdate(ctx context.Context, tx Transaction, orgId uuid.UUID, slug models.Role) (models.RbacRole, error)
	CreateRole(ctx context.Context, exec Executor, orgId uuid.UUID, slug, name string) (models.RbacRole, error)
	UpdateRolePermissions(ctx context.Context, exec Executor, orgId uuid.UUID, slug models.Role, permissions []models.Permission) error
	ReplaceUserRoleBindings(ctx context.Context, tx Transaction, orgId uuid.UUID, userId string, bindings []models.RoleBinding) error
	ListUserOrganizationRoleBindings(ctx context.Context, exec Executor, orgId uuid.UUID, userId string) ([]models.RoleBinding, error)
	ReplaceUserOrganizationRoleBindings(ctx context.Context, tx Transaction, orgId uuid.UUID, userId string, bindings []models.RoleBinding) error
}

func (repo *MarbleDbRepository) CreateUser(ctx context.Context, tx Transaction, createUser models.CreateUser) (string, error) {
	userId := pure_utils.NewId().String()
	if err := validateMarbleDbExecutor(tx); err != nil {
		return "", err
	}

	err := ExecBuilder(
		ctx,
		tx,
		NewQueryBuilder().Insert(dbmodels.TABLE_USERS).
			Columns(
				"id",
				"email",
				"role",
				"organization_id",
				"first_name",
				"last_name",
			).
			Values(
				userId,
				createUser.Email,
				models.LegacyRoleValue(createUser.RoleBindings),
				createUser.OrganizationId,
				createUser.FirstName,
				createUser.LastName,
			),
	)
	if err != nil {
		return "", err
	}

	if err := repo.ReplaceUserRoleBindings(ctx, tx, createUser.OrganizationId, userId, createUser.RoleBindings); err != nil {
		return "", err
	}

	return userId, nil
}

func (repo *MarbleDbRepository) UpdateUser(ctx context.Context, tx Transaction, updateUser models.UpdateUser) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_USERS).Where(squirrel.Eq{"id": updateUser.UserId})
	updateProfile := false

	if updateUser.Email != nil {
		query = query.Set("email", *updateUser.Email)
		updateProfile = true
	}
	if updateUser.FirstName != nil {
		query = query.Set("first_name", *updateUser.FirstName)
		updateProfile = true
	}
	if updateUser.LastName != nil {
		query = query.Set("last_name", *updateUser.LastName)
		updateProfile = true
	}
	if updateProfile {
		if err := ExecBuilder(ctx, tx, query); err != nil {
			return err
		}
	}

	if updateUser.RoleBindings != nil {
		var persistedOrgId *uuid.UUID

		query, args, err := NewQueryBuilder().
			Select("organization_id").
			From(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"id": updateUser.UserId}).
			ToSql()
		if err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, query, args...).Scan(&persistedOrgId); err != nil {
			return err
		}

		orgId := uuid.Nil

		if persistedOrgId != nil {
			orgId = *persistedOrgId
		}

		if err := repo.ReplaceUserRoleBindings(ctx, tx, orgId, updateUser.UserId, *updateUser.RoleBindings); err != nil {
			return err
		}

		// TODO(MAR-2251): remove users.role once legacy JWTs have expired.
		if err := ExecBuilder(ctx, tx, NewQueryBuilder().
			Update(dbmodels.TABLE_USERS).
			Set("role", models.LegacyRoleValue(*updateUser.RoleBindings)).
			Where(squirrel.Eq{"id": updateUser.UserId})); err != nil {
			return err
		}
	}

	return tx.Cache(ctx).Exec(func(c *redis.Client) error {
		return c.Del(ctx, tx.Cache(ctx).Key("user", updateUser.UserId)).Err()
	})
}

func (repo *MarbleDbRepository) DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"id": userID}).
			Set("deleted_at", squirrel.Expr("NOW()")),
	)
	if err != nil {
		return err
	}
	if err := ExecBuilder(ctx, exec,
		NewQueryBuilder().
			Update(dbmodels.TABLE_GRANTS).
			Set("revoked_at", squirrel.Expr("NOW()")).
			Where(squirrel.Eq{
				"principal_type": "user",
				"principal_id":   userID,
				"revoked_at":     nil,
			}),
	); err != nil {
		return err
	}

	return exec.Cache(ctx).Exec(func(c *redis.Client) error {
		return c.Del(ctx, exec.Cache(ctx).Key("user", string(userID))).Err()
	})
}

func (repo *MarbleDbRepository) DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(ctx, exec, NewQueryBuilder().Delete("grants").Where("organization_id = ?", organizationId))
	return err
}

func (repo *MarbleDbRepository) UserById(ctx context.Context, exec Executor, userId string) (models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.User{}, err
	}

	// Only the user row is cached: role bindings change through many paths
	// (grants, expiry) that do not invalidate it, so they are always loaded.
	user, err := RedisLoadModel[models.User](ctx, exec.Cache(ctx), exec.Cache(ctx).Key("user", userId))
	if err != nil {
		user, err = SqlToModel(
			ctx,
			exec,
			NewQueryBuilder().
				Select(dbmodels.UserFields...).
				From(dbmodels.TABLE_USERS).
				Where(squirrel.Eq{"id": userId}).
				Where("deleted_at IS NULL").
				OrderBy("id"),
			dbmodels.AdaptUser,
		)
		if err != nil {
			return user, err
		}

		user.RoleBindings = nil
		_ = exec.Cache(ctx).SaveModel(ctx, exec, exec.Cache(ctx).Key("user", userId), user, time.Hour)
	}

	bindings, err := repo.ListUserRoleBindings(ctx, exec, user.OrganizationId, string(user.UserId))
	if err != nil {
		return user, err
	}

	user.RoleBindings = bindings

	return user, nil
}

// ListUsers lists all users, or with orgId the users holding an active grant on that organization.
func (repo *MarbleDbRepository) ListUsers(ctx context.Context, exec Executor, orgId *uuid.UUID) ([]models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if orgId != nil {
		return repo.listOrganizationUsers(ctx, exec, *orgId)
	}

	query := NewQueryBuilder().
		Select(dbmodels.UserFields...).
		From(dbmodels.TABLE_USERS).
		Where("deleted_at IS NULL").
		OrderBy("id")

	users, err := SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptUser,
	)
	if err != nil {
		return nil, err
	}

	bindings, err := repo.listUsersRoleBindings(ctx, exec,
		pure_utils.Map(users, func(user models.User) string { return string(user.UserId) }))
	if err != nil {
		return nil, err
	}

	for idx := range users {
		users[idx].RoleBindings = bindings[string(users[idx].UserId)]
	}

	return users, nil
}

// listOrganizationUsers returns the users holding an active organization-scoped grant on the
// organization, whatever their home organization. Each user is seen from that organization:
// their organization is the requested one and their role bindings are the ones granted on it.
func (repo *MarbleDbRepository) listOrganizationUsers(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.User, error) {
	query := NewQueryBuilder().
		Select(columnsNames("u", dbmodels.UserFields)...).
		From(dbmodels.TABLE_USERS+" u").
		Where(`EXISTS (
			SELECT 1 FROM active_grants g
			WHERE g.principal_type = 'user'
			AND g.principal_id = u.id::text
			AND g.principal_authority = 'marble'
			AND g.organization_id = ?
		)`, orgId).
		Where("u.deleted_at IS NULL").
		OrderBy("u.id")

	users, err := SqlToListOfModels(ctx, exec, query, dbmodels.AdaptUser)
	if err != nil {
		return nil, err
	}

	bindings, err := repo.listUsersScopedRoleBindings(ctx, exec, homeScope(orgId),
		pure_utils.Map(users, func(user models.User) string { return string(user.UserId) }))
	if err != nil {
		return nil, err
	}

	for idx := range users {
		users[idx].OrganizationId = orgId
		users[idx].RoleBindings = bindings[string(users[idx].UserId)]
	}

	return users, nil
}

func (repo *MarbleDbRepository) UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	user, err := SqlToOptionalModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.UserFields...).
			From(dbmodels.TABLE_USERS).
			Where("email = ?", email).
			Where("deleted_at IS NULL").
			OrderBy("id"),
		dbmodels.AdaptUser,
	)
	if err != nil || user == nil {
		return user, err
	}

	bindings, err := repo.ListUserRoleBindings(ctx, exec, user.OrganizationId, string(user.UserId))
	if err != nil {
		return nil, err
	}

	user.RoleBindings = bindings

	return user, nil
}

func (repo *MarbleDbRepository) HasUsers(ctx context.Context, exec Executor) (bool, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return false, err
	}
	var exists bool
	err := exec.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+dbmodels.TABLE_USERS+" LIMIT 1)").Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (repo *MarbleDbRepository) GetRoleBySlug(ctx context.Context, exec Executor, orgId uuid.UUID, slug models.Role) (models.RbacRole, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.RbacRole{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectRoleColumn...).
			From(dbmodels.TABLE_ROLES).
			Where(squirrel.Eq{"org_id": orgId, "slug": slug}),
		dbmodels.AdaptRole,
	)
}

func (repo *MarbleDbRepository) GetRoleBySlugForUpdate(ctx context.Context, tx Transaction, orgId uuid.UUID, slug models.Role) (models.RbacRole, error) {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return models.RbacRole{}, err
	}

	return SqlToModel(
		ctx,
		tx,
		NewQueryBuilder().
			Select(dbmodels.SelectRoleColumn...).
			From(dbmodels.TABLE_ROLES).
			Where(squirrel.Eq{"org_id": orgId, "slug": slug}).
			Suffix("FOR UPDATE"),
		dbmodels.AdaptRole,
	)
}

func (repo *MarbleDbRepository) ListRoles(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.RbacRole, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		squirrel.Sqlizer(NewQueryBuilder().
			Select(dbmodels.SelectRoleColumn...).
			From(dbmodels.TABLE_ROLES).
			Where("org_id = ?", orgId).
			OrderBy("name")),
		dbmodels.AdaptRole,
	)
}

func (repo *MarbleDbRepository) CreateRole(ctx context.Context, exec Executor, orgId uuid.UUID, slug, name string) (models.RbacRole, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.RbacRole{}, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_ROLES).
		Columns("id", "org_id", "slug", "name").
		Values(pure_utils.NewId(), orgId, slug, name).
		Suffix(fmt.Sprintf("returning %s", strings.Join(dbmodels.SelectRoleColumn, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptRole)
}

func (repo *MarbleDbRepository) UpdateRolePermissions(ctx context.Context, exec Executor, orgId uuid.UUID, slug models.Role, permissions []models.Permission) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query, args, err := NewQueryBuilder().
		Update(dbmodels.TABLE_ROLES).
		Set("permissions", permissions).
		Where(squirrel.Eq{"org_id": orgId, "slug": slug}).
		ToSql()
	if err != nil {
		return err
	}

	result, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return models.NotFoundError
	}

	return nil
}
