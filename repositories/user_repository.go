package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/redis/go-redis/v9"

	"github.com/google/uuid"
)

type UserRepository interface {
	CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (string, error)
	UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error
	DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error
	DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error
	UserById(ctx context.Context, exec Executor, userId string) (models.User, error)
	ListUsers(ctx context.Context, exec Executor, organizationId *uuid.UUID) ([]models.User, error)
	UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error)
	HasUsers(ctx context.Context, exec Executor) (bool, error)

	ListRoles(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.RbacRole, error)
	GetRoleBySlug(ctx context.Context, exec Executor, orgId uuid.UUID, slug models.Role) (models.RbacRole, error)
	CreateRole(ctx context.Context, exec Executor, orgId uuid.UUID, slug, name string) (models.RbacRole, error)
	UpdateRolePermissions(ctx context.Context, exec Executor, orgId uuid.UUID, slug models.Role, permissions []dto.RoleGrantPermission) error
	ReplaceUserRoleBindings(ctx context.Context, exec Executor, orgId uuid.UUID, userId string, bindings []models.RoleBinding) error
}

func (repo *MarbleDbRepository) CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (string, error) {
	userId := pure_utils.NewId().String()

	if err := validateMarbleDbExecutor(exec); err != nil {
		return "", err
	}

	err := ExecBuilder(
		ctx,
		exec,
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

	if err := repo.ReplaceUserRoleBindings(ctx, exec, createUser.OrganizationId, userId, createUser.RoleBindings); err != nil {
		return "", err
	}

	return userId, nil
}

func (repo *MarbleDbRepository) UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
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
		if err := ExecBuilder(ctx, exec, query); err != nil {
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

		if err := exec.QueryRow(ctx, query, args...).Scan(&persistedOrgId); err != nil {
			return err
		}

		orgId := uuid.Nil

		if persistedOrgId != nil {
			orgId = *persistedOrgId
		}

		if err := repo.ReplaceUserRoleBindings(ctx, exec, orgId, updateUser.UserId, *updateUser.RoleBindings); err != nil {
			return err
		}
	}

	return exec.Cache(ctx).Exec(func(c *redis.Client) error {
		return c.Del(ctx, exec.Cache(ctx).Key("user", updateUser.UserId)).Err()
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

	return exec.Cache(ctx).Exec(func(c *redis.Client) error {
		return c.Del(ctx, exec.Cache(ctx).Key("user", string(userID))).Err()
	})
}

func (repo *MarbleDbRepository) DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Delete(dbmodels.TABLE_USERS).Where("organization_id = ?", organizationId),
	)
	return err
}

func (repo *MarbleDbRepository) UserById(ctx context.Context, exec Executor, userId string) (models.User, error) {
	if user, err := RedisLoadModel[models.User](ctx, exec.Cache(ctx),
		exec.Cache(ctx).Key("user", userId)); err == nil {
		return user, nil
	}

	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.User{}, err
	}

	user, err := SqlToModel(
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

	bindings, err := repo.ListUserRoleBindings(ctx, exec, string(user.UserId))
	if err != nil {
		return user, err
	}

	user.RoleBindings = bindings

	_ = exec.Cache(ctx).SaveModel(ctx, exec, exec.Cache(ctx).Key("user", userId), user, time.Hour)

	return user, nil
}

func (repo *MarbleDbRepository) ListUsers(ctx context.Context, exec Executor, orgId *uuid.UUID) ([]models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.UserFields...).
		From(dbmodels.TABLE_USERS).
		Where("deleted_at IS NULL").
		OrderBy("id")

	if orgId != nil {
		query = query.Where(squirrel.Eq{"organization_id": *orgId})
	}

	users, err := SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptUser,
	)
	if err != nil {
		return nil, err
	}

	for idx := range users {
		bindings, err := repo.ListUserRoleBindings(ctx, exec, string(users[idx].UserId))
		if err != nil {
			return nil, err
		}

		users[idx].RoleBindings = bindings
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

	bindings, err := repo.ListUserRoleBindings(ctx, exec, string(user.UserId))
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
			From(dbmodels.TABLE_ROLES+" r").
			Where(squirrel.Eq{"org_id": orgId, "slug": slug}),
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
		NewQueryBuilder().
			Select(dbmodels.SelectRoleColumn...).
			From(dbmodels.TABLE_ROLES+" r").
			Where("org_id = ?", orgId),
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

func (repo *MarbleDbRepository) UpdateRolePermissions(ctx context.Context, exec Executor, orgId uuid.UUID, slug models.Role, permissions []dto.RoleGrantPermission) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	names := pure_utils.Map(permissions, func(permission dto.RoleGrantPermission) string {
		return permission.Name
	})

	query, args, err := NewQueryBuilder().
		Update(dbmodels.TABLE_ROLES).
		Set("permissions", names).
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
