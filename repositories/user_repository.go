package repositories

import (
	"context"
	"slices"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type UserRepository interface {
	ListUsersByTenant(context.Context, Executor, uuid.UUID) ([]models.User, error)
	CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (string, error)
	UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error
	DeleteUser(ctx context.Context, exec Executor, userID models.UserId) error
	DeleteUsersOfOrganization(ctx context.Context, exec Executor, organizationId uuid.UUID) error
	UserById(ctx context.Context, exec Executor, userId string) (models.User, error)
	ListUsers(ctx context.Context, exec Executor, organizationId *uuid.UUID) ([]models.User, error)
	UserByEmail(ctx context.Context, exec Executor, email string) (*models.User, error)
	HasUsers(ctx context.Context, exec Executor) (bool, error)
}

func (repo *MarbleDbRepository) ListUsersByTenant(ctx context.Context, exec Executor, tenantID uuid.UUID) ([]models.User, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	return SqlToListOfModels(ctx, exec, NewQueryBuilder().Select(dbmodels.UserFields...).From(dbmodels.TABLE_USERS).
		Where(squirrel.Eq{"tenant_id": tenantID, "deleted_at": nil}).OrderBy("email", "id"), dbmodels.AdaptUser)
}

func (repo *MarbleDbRepository) CreateUser(ctx context.Context, exec Executor, createUser models.CreateUser) (string, error) {
	userId := pure_utils.NewId().String()
	if err := validateMarbleDbExecutor(exec); err != nil {
		return "", err
	}
	if createUser.OrganizationId != uuid.Nil {
		var tenantID uuid.UUID
		query, args, err := NewQueryBuilder().Select("tenant_id").From("organizations").
			Where(squirrel.Eq{"id": createUser.OrganizationId, "deleted_at": nil}).ToSql()
		if err != nil {
			return "", err
		}
		if err := exec.QueryRow(ctx, query, args...).Scan(&tenantID); err != nil {
			return "", err
		}
		if createUser.TenantId != uuid.Nil && createUser.TenantId != tenantID {
			return "", models.ForbiddenError
		}
		createUser.TenantId = tenantID
	}
	var tenantID *uuid.UUID
	if createUser.Role != models.MARBLE_ADMIN && createUser.TenantId != uuid.Nil {
		tenantID = &createUser.TenantId
		query, args, err := NewQueryBuilder().Select("id").From("tenants").
			Where(squirrel.Eq{"id": tenantID, "deleted_at": nil}).Suffix("FOR SHARE").ToSql()
		if err != nil {
			return "", err
		}
		var lockedID uuid.UUID
		if err := exec.QueryRow(ctx, query, args...).Scan(&lockedID); err != nil {
			return "", err
		}
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_USERS).
			Columns(
				"id",
				"email",
				"role",
				"tenant_id",
				"first_name",
				"last_name",
			).
			Values(
				userId,
				createUser.Email,
				int(createUser.Role),
				tenantID,
				createUser.FirstName,
				createUser.LastName,
			),
	)
	if err != nil {
		return "", err
	}

	if createUser.Role == models.MARBLE_ADMIN {
		return userId, ExecBuilder(ctx, exec, NewQueryBuilder().
			Insert(dbmodels.TABLE_GRANTS).
			Columns("id", "principal_type", "principal_id", "principal_authority", "role").
			Values(pure_utils.NewId(), "user", userId, "marble", createUser.Role.String()).
			Suffix("ON CONFLICT DO NOTHING"))
	}
	if createUser.OrganizationId == uuid.Nil || createUser.Role == models.NO_ROLE {
		return userId, nil
	}

	return userId, ExecBuilder(ctx, exec, NewQueryBuilder().
		Insert(dbmodels.TABLE_GRANTS).
		Columns("id", "principal_type", "principal_id", "principal_authority", "organization_id", "role").
		Values(pure_utils.NewId(), "user", userId, "marble", createUser.OrganizationId, createUser.Role.String()).
		Suffix("ON CONFLICT DO NOTHING"))
}

func (repo *MarbleDbRepository) UpdateUser(ctx context.Context, exec Executor, updateUser models.UpdateUser) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	if updateUser.Email == nil && updateUser.Role == nil && updateUser.FirstName == nil && updateUser.LastName == nil {
		return nil
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_USERS).Where(squirrel.Eq{"id": updateUser.UserId})
	var previousRole models.Role
	var previousOrganizationID *uuid.UUID
	if updateUser.OrganizationId != uuid.Nil {
		previousOrganizationID = &updateUser.OrganizationId
	}
	if updateUser.Role != nil && *updateUser.Role != models.NO_ROLE {
		var role int
		selectQuery, args, err := NewQueryBuilder().
			Select("role").
			From(dbmodels.TABLE_USERS).
			Where(squirrel.Eq{"id": updateUser.UserId}).
			ToSql()
		if err != nil {
			return err
		}
		if err := exec.QueryRow(ctx, selectQuery, args...).Scan(&role); err != nil {
			return err
		}
		previousRole = models.Role(role)
		if previousRole == models.MARBLE_ADMIN && *updateUser.Role != models.MARBLE_ADMIN {
			return errors.Wrap(models.ForbiddenError, "manage platform administrators separately from organization grants")
		}
		if previousOrganizationID == nil && *updateUser.Role != models.MARBLE_ADMIN {
			return errors.Errorf("cannot assign an organization role to a platform user without an organization")
		}
	}

	if updateUser.Email != nil {
		query = query.Set("email", *updateUser.Email)
	}
	if updateUser.Role != nil && *updateUser.Role != models.NO_ROLE {
		query = query.Set("role", int(*updateUser.Role))
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
	if updateUser.Role != nil && *updateUser.Role != models.NO_ROLE {
		revokeQuery := NewQueryBuilder().
			Update(dbmodels.TABLE_GRANTS).
			Set("revoked_at", squirrel.Expr("NOW()")).
			Where(squirrel.Eq{
				"principal_type":      "user",
				"principal_id":        updateUser.UserId,
				"principal_authority": "marble",
				"revoked_at":          nil,
			})
		if previousOrganizationID == nil {
			revokeQuery = revokeQuery.Where(squirrel.Eq{
				"role":            previousRole.String(),
				"organization_id": nil,
				"tenant_id":       nil,
			})
		} else {
			revokeQuery = revokeQuery.Where(squirrel.Eq{"organization_id": *previousOrganizationID})
		}
		if err := ExecBuilder(ctx, exec, revokeQuery); err != nil {
			return err
		}
		grantQuery := NewQueryBuilder().Insert(dbmodels.TABLE_GRANTS)
		if *updateUser.Role == models.MARBLE_ADMIN {
			grantQuery = grantQuery.
				Columns("id", "principal_type", "principal_id", "principal_authority", "role").
				Values(pure_utils.NewId(), "user", updateUser.UserId, "marble", updateUser.Role.String())
		} else {
			grantQuery = grantQuery.
				Columns("id", "principal_type", "principal_id", "principal_authority", "organization_id", "role").
				Values(pure_utils.NewId(), "user", updateUser.UserId, "marble", previousOrganizationID, updateUser.Role.String())
		}
		if err := ExecBuilder(ctx, exec, grantQuery.Suffix("ON CONFLICT DO NOTHING")); err != nil {
			return err
		}
	}

	return nil
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
				"principal_type":      "user",
				"principal_authority": "marble",
				"principal_id":        userID,
				"revoked_at":          nil,
			}),
	); err != nil {
		return err
	}

	return nil
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

	query := NewQueryBuilder().Select(dbmodels.UserFields...).From(dbmodels.TABLE_USERS).
		Where(squirrel.Eq{"id": userId}).Where("deleted_at IS NULL").OrderBy("id")
	if _, transactional := exec.(Transaction); transactional {
		query = query.Suffix("FOR UPDATE")
	}
	user, err := SqlToModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptUser,
	)
	if err != nil {
		return user, err
	}
	memberships, err := repo.ListOrganizationsForUser(ctx, exec, userId)
	if err != nil {
		return models.User{}, err
	}
	for _, membership := range memberships {
		user.OrganizationIds = append(user.OrganizationIds, membership.Organization.Id)
		if user.OrganizationRoles == nil {
			user.OrganizationRoles = make(map[uuid.UUID][]models.Role)
		}
		user.OrganizationRoles[membership.Organization.Id] = membership.Roles
	}

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
		grants := NewQueryBuilder().Select("1").From("active_grants g").
			Where(squirrel.Eq{"g.principal_type": "user", "g.principal_authority": "marble", "g.organization_id": *orgId}).
			Where("g.principal_id = users.id::text").PlaceholderFormat(squirrel.Question)
		query = query.Where(squirrel.Expr("EXISTS (?)", grants))
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
	ids := make([]string, len(users))
	for i := range users {
		ids[i] = string(users[i].UserId)
	}
	grants, err := repo.ListOrganizationGrantsByUser(ctx, exec, ids)
	if err != nil {
		return nil, err
	}
	for i := range users {
		for _, grant := range grants[ids[i]] {
			users[i].OrganizationIds = append(users[i].OrganizationIds, grant.OrganizationId)
			if orgId != nil && grant.OrganizationId == *orgId && users[i].Role != models.MARBLE_ADMIN && slices.Contains(models.GetValidOrganizationGrantRoles(), grant.Role) {
				users[i].Role = grant.Role
			}
		}
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
	loaded, err := repo.UserById(ctx, exec, string(user.UserId))
	if err != nil {
		return nil, err
	}
	return &loaded, nil
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
