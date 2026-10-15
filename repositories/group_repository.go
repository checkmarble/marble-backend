package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type GroupRepository interface {
	ListGroups(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.Group, error)
	GetGroup(ctx context.Context, exec Executor, orgId, groupId uuid.UUID) (models.Group, error)
	LockGroup(ctx context.Context, tx Transaction, orgId, groupId uuid.UUID) error
	CreateGroup(ctx context.Context, exec Executor, group models.Group) error
	RenameGroup(ctx context.Context, exec Executor, orgId, groupId uuid.UUID, name string) error
	DeleteGroup(ctx context.Context, tx Transaction, orgId, groupId uuid.UUID) error
	ReplaceGroupMembers(ctx context.Context, tx Transaction, groupId uuid.UUID, memberIds []models.UserId) error
	ReplaceGroupRoleBindings(ctx context.Context, tx Transaction, orgId, groupId uuid.UUID,
		bindings []models.RoleBinding) error
	FilterOrganizationTenantUsers(ctx context.Context, exec Executor, orgId uuid.UUID,
		userIds []models.UserId) ([]models.UserId, error)
}

func groupQuery(orgId uuid.UUID) squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectGroupColumns...).
		From(dbmodels.TABLE_GROUPS).
		Where(squirrel.Eq{"organization_id": orgId, "deleted_at": nil})
}

func (repo *MarbleDbRepository) ListGroups(ctx context.Context, exec Executor, orgId uuid.UUID) ([]models.Group, error) {
	return repo.listGroups(ctx, exec, orgId, groupQuery(orgId).OrderBy("name", "id"))
}

func (repo *MarbleDbRepository) GetGroup(ctx context.Context, exec Executor, orgId, groupId uuid.UUID) (models.Group, error) {
	groups, err := repo.listGroups(ctx, exec, orgId, groupQuery(orgId).Where(squirrel.Eq{"id": groupId}))
	if err != nil {
		return models.Group{}, err
	}
	if len(groups) == 0 {
		return models.Group{}, models.NotFoundError
	}

	return groups[0], nil
}

// listGroups returns the groups selected by the query, with their role
// bindings and members.
func (repo *MarbleDbRepository) listGroups(
	ctx context.Context, exec Executor, orgId uuid.UUID, query squirrel.SelectBuilder,
) ([]models.Group, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	groups, err := SqlToListOfModels(ctx, exec, query, dbmodels.AdaptGroup)
	if err != nil || len(groups) == 0 {
		return groups, err
	}

	ids := make([]uuid.UUID, len(groups))
	principalIds := make([]string, len(groups))
	byId := make(map[uuid.UUID]*models.Group, len(groups))
	for i := range groups {
		ids[i] = groups[i].Id
		principalIds[i] = groups[i].Id.String()
		byId[groups[i].Id] = &groups[i]
	}

	bindings, err := SqlToListOfModels(ctx, exec,
		roleBindingsQuery(dbmodels.GrantPrincipalGroup, principalIds).Where(organizationScope(orgId).where("g.")),
		dbmodels.AdaptRoleBinding)
	if err != nil {
		return nil, err
	}

	for _, binding := range bindings {
		if binding.GroupId != nil {
			byId[*binding.GroupId].RoleBindings = append(byId[*binding.GroupId].RoleBindings, binding)
		}
	}

	memberships, err := SqlToListOfModels(ctx, exec,
		NewQueryBuilder().
			Select(dbmodels.SelectGroupMembershipColumns...).
			From(dbmodels.TABLE_GROUP_MEMBERSHIPS).
			Where(squirrel.Eq{"group_id": ids}).
			OrderBy("user_id"),
		dbmodels.AdaptGroupMembership)
	if err != nil {
		return nil, err
	}

	for _, membership := range memberships {
		byId[membership.GroupId].MemberIds = append(byId[membership.GroupId].MemberIds, membership.UserId)
	}

	return groups, nil
}

// LockGroup locks an active group until the end of the transaction, so that
// changes to its members and role bindings, which are authorized against each
// other, are serialized.
func (repo *MarbleDbRepository) LockGroup(ctx context.Context, tx Transaction, orgId, groupId uuid.UUID) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	groups, err := SqlToListOfModels(ctx, tx,
		groupQuery(orgId).Where(squirrel.Eq{"id": groupId}).Suffix("FOR UPDATE"),
		dbmodels.AdaptGroup)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return models.NotFoundError
	}

	return nil
}

func (repo *MarbleDbRepository) CreateGroup(ctx context.Context, exec Executor, group models.Group) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	return ExecBuilder(ctx, exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_GROUPS).
			Columns("id", "organization_id", "name").
			Values(group.Id, group.OrganizationId, group.Name))
}

func (repo *MarbleDbRepository) RenameGroup(ctx context.Context, exec Executor, orgId, groupId uuid.UUID, name string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query, args, err := NewQueryBuilder().
		Update(dbmodels.TABLE_GROUPS).
		Set("name", name).
		Where(squirrel.Eq{"id": groupId, "organization_id": orgId, "deleted_at": nil}).
		ToSql()
	if err != nil {
		return err
	}

	tag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return models.NotFoundError
	}

	return nil
}

// DeleteGroup soft-deletes a group, revoking its grants and removing its
// members.
func (repo *MarbleDbRepository) DeleteGroup(ctx context.Context, tx Transaction, orgId, groupId uuid.UUID) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	if err := ExecBuilder(ctx, tx,
		NewQueryBuilder().
			Update(dbmodels.TABLE_GRANTS).
			Set("revoked_at", squirrel.Expr("now()")).
			Where(squirrel.Eq{
				"principal_type":  dbmodels.GrantPrincipalGroup,
				"principal_id":    groupId.String(),
				"organization_id": orgId,
				"revoked_at":      nil,
			})); err != nil {
		return err
	}

	if err := ExecBuilder(ctx, tx,
		NewQueryBuilder().
			Delete(dbmodels.TABLE_GROUP_MEMBERSHIPS).
			Where(squirrel.Eq{"group_id": groupId})); err != nil {
		return err
	}

	return ExecBuilder(ctx, tx,
		NewQueryBuilder().
			Update(dbmodels.TABLE_GROUPS).
			Set("deleted_at", squirrel.Expr("now()")).
			Where(squirrel.Eq{"id": groupId, "organization_id": orgId, "deleted_at": nil}))
}

func (repo *MarbleDbRepository) ReplaceGroupMembers(
	ctx context.Context, tx Transaction, groupId uuid.UUID, memberIds []models.UserId,
) error {
	if err := validateMarbleDbExecutor(tx); err != nil {
		return err
	}

	if err := ExecBuilder(ctx, tx,
		NewQueryBuilder().
			Delete(dbmodels.TABLE_GROUP_MEMBERSHIPS).
			Where(squirrel.Eq{"group_id": groupId})); err != nil {
		return err
	}

	if len(memberIds) == 0 {
		return nil
	}

	insert := NewQueryBuilder().
		Insert(dbmodels.TABLE_GROUP_MEMBERSHIPS).
		Columns("group_id", "user_id")
	for _, userId := range memberIds {
		insert = insert.Values(groupId, string(userId))
	}

	return ExecBuilder(ctx, tx, insert)
}

func (repo *MarbleDbRepository) ReplaceGroupRoleBindings(
	ctx context.Context, tx Transaction, orgId, groupId uuid.UUID, bindings []models.RoleBinding,
) error {
	return repo.replaceRoleBindings(ctx, tx, organizationScope(orgId),
		dbmodels.GrantPrincipalGroup, groupId.String(), bindings)
}

// FilterOrganizationTenantUsers returns, among the given users, the active ones
// whose home organization is in the tenant of the organization.
func (repo *MarbleDbRepository) FilterOrganizationTenantUsers(
	ctx context.Context, exec Executor, orgId uuid.UUID, userIds []models.UserId,
) ([]models.UserId, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}
	if len(userIds) == 0 {
		return []models.UserId{}, nil
	}

	query := NewQueryBuilder().
		Select("u.id::text").
		From(dbmodels.TABLE_USERS+" u").
		Join("organizations home ON home.id = u.organization_id").
		Join("organizations o ON o.tenant_id = home.tenant_id").
		Where(squirrel.Eq{
			"u.id":         pure_utils.Map(userIds, func(id models.UserId) string { return string(id) }),
			"u.deleted_at": nil,
			"o.id":         orgId,
		})

	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.UserId, error) {
		var id string
		err := row.Scan(&id)

		return models.UserId(id), err
	})
}
