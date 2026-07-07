package repositories

import (
	"context"
	"encoding/json"
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
	CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (string, error)
	UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error
	DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error
	DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error
	UserById(ctx context.Context, exec Executor, userId string) (models.User, error)
	ListUsers(ctx context.Context, exec Executor, organizationId *uuid.UUID) ([]models.User, error)
	UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error)
	HasUsers(ctx context.Context, exec Executor) (bool, error)

	ListRoles(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.RbacRole, error)
	GetRole(ctx context.Context, exec Executor, orgId, roleId uuid.UUID) (models.RbacRole, error)
	CreateRole(ctx context.Context, exec Executor, orgId uuid.UUID, name string) (models.RbacRole, error)
	UpdateRolePermissions(ctx context.Context, exec Executor, orgId, roleId uuid.UUID, permissions []string) error
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
				"roles",
				"organization_id",
				"first_name",
				"last_name",
			).
			Values(
				userId,
				createUser.Email,
				0,
				createUser.Roles,
				createUser.OrganizationId,
				createUser.FirstName,
				createUser.LastName,
			),
	)
	return userId, err
}

func (repo *MarbleDbRepository) UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_USERS).Where(squirrel.Eq{"id": updateUser.UserId})

	if updateUser.Email != nil {
		query = query.Set("email", *updateUser.Email)
	}
	if updateUser.Roles != nil {
		query = query.Set("roles", *updateUser.Roles)
	}
	if updateUser.FirstName != nil {
		query = query.Set("first_name", *updateUser.FirstName)
	}
	if updateUser.LastName != nil {
		query = query.Set("last_name", *updateUser.LastName)
	}

	if err := ExecBuilder(ctx, exec, query); err != nil {
		return err
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

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptUser,
	)
}

func (repo *MarbleDbRepository) UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToOptionalModel(
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

func (repo *MarbleDbRepository) GetRole(ctx context.Context, exec Executor, orgId, roleId uuid.UUID) (models.RbacRole, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.RbacRole{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectRoleColumn...).
			Column(fmt.Sprintf(`(select array_agg(row(%s)) from permissions p where p.role_id = r.id) as permissions`, strings.Join(dbmodels.SelectPermissionColumn, ","))).
			From(dbmodels.TABLE_ROLES+" r").
			Where("org_id = ? and id = ?", orgId, roleId),
		dbmodels.AdaptRoleWithPermissions,
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
			Column(fmt.Sprintf(`(select array_agg(row(%s)) from permissions p where p.role_id = r.id) as permissions`, strings.Join(dbmodels.SelectPermissionColumn, ","))).
			From(dbmodels.TABLE_ROLES+" r").
			Where("org_id = ?", orgId),
		dbmodels.AdaptRoleWithPermissions,
	)
}

func (repo *MarbleDbRepository) CreateRole(ctx context.Context, exec Executor, orgId uuid.UUID, name string) (models.RbacRole, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.RbacRole{}, err
	}

	name = "org/" + name

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_ROLES).
		Columns("id", "org_id", "name").
		Values(pure_utils.NewId(), orgId, name).
		Suffix(fmt.Sprintf("returning %s", strings.Join(dbmodels.SelectRoleColumn, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptRole)
}

func (repo *MarbleDbRepository) UpdateRolePermissions(ctx context.Context, exec Executor, orgId, roleId uuid.UUID, permissions []string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	inputs := make([]dbmodels.DbPermission, len(permissions))

	for idx, perm := range permissions {
		inputs[idx] = dbmodels.DbPermission{
			Id:     pure_utils.NewId(),
			OrgId:  orgId,
			RoleId: roleId,
			Name:   perm,
		}
	}

	jsonb, err := json.Marshal(inputs)
	if err != nil {
		return err
	}

	sql := fmt.Sprintf(`
		merge into permissions as p
		using jsonb_populate_recordset(null::permissions, $1::jsonb) as s (%[1]s)
		on p.role_id = s.role_id::uuid and p.name = s.name
		when not matched then
		  insert (%[1]s) values (s.id, s.org_id::uuid, s.role_id::uuid, s.name, s.condition)
		when not matched by source and p.org_id = $2::uuid and p.role_id = $3 then
		  delete;
	`, strings.Join(dbmodels.SelectPermissionColumn, ","))

	_, err = exec.Exec(ctx, sql, jsonb, orgId, roleId)

	return err
}
