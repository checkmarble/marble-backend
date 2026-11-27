package usecases

import (
	"context"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type GroupUseCase struct {
	enforceSecurity    security.EnforceSecurityGroup
	credentials        models.Credentials
	executorFactory    executor_factory.ExecutorFactory
	transactionFactory executor_factory.TransactionFactory
	groupRepository    repositories.GroupRepository
	roleRepository     customRoleGetter
}

func (uc GroupUseCase) List(ctx context.Context) ([]models.Group, error) {
	if err := uc.enforceSecurity.ManageGroups(); err != nil {
		return nil, err
	}

	groups, err := uc.groupRepository.ListGroups(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId())
	if err != nil {
		return nil, errors.Wrap(err, "list groups")
	}

	return groups, nil
}

func (uc GroupUseCase) Get(ctx context.Context, groupId uuid.UUID) (models.Group, error) {
	if err := uc.enforceSecurity.ManageGroups(); err != nil {
		return models.Group{}, err
	}

	group, err := uc.groupRepository.GetGroup(ctx, uc.executorFactory.NewExecutor(), uc.enforceSecurity.OrgId(), groupId)
	if err != nil {
		return models.Group{}, errors.Wrap(err, "get group")
	}

	return group, nil
}

func (uc GroupUseCase) Create(ctx context.Context, name string) (models.Group, error) {
	if err := uc.enforceSecurity.ManageGroups(); err != nil {
		return models.Group{}, err
	}

	name, err := validateGroupName(name)
	if err != nil {
		return models.Group{}, err
	}

	group := models.Group{
		Id:             pure_utils.NewId(),
		OrganizationId: uc.enforceSecurity.OrgId(),
		Name:           name,
	}

	err = uc.groupRepository.CreateGroup(ctx, uc.executorFactory.NewExecutor(), group)
	if repositories.IsUniqueViolationError(err) {
		return models.Group{}, errors.Wrap(models.ConflictError, "group name already exists")
	}
	if err != nil {
		return models.Group{}, errors.Wrap(err, "create group")
	}

	return group, nil
}

func (uc GroupUseCase) Rename(ctx context.Context, groupId uuid.UUID, name string) (models.Group, error) {
	if err := uc.enforceSecurity.ManageGroups(); err != nil {
		return models.Group{}, err
	}

	name, err := validateGroupName(name)
	if err != nil {
		return models.Group{}, err
	}

	orgId := uc.enforceSecurity.OrgId()
	exec := uc.executorFactory.NewExecutor()

	err = uc.groupRepository.RenameGroup(ctx, exec, orgId, groupId, name)
	if repositories.IsUniqueViolationError(err) {
		return models.Group{}, errors.Wrap(models.ConflictError, "group name already exists")
	}
	if err != nil {
		return models.Group{}, errors.Wrap(err, "rename group")
	}

	group, err := uc.groupRepository.GetGroup(ctx, exec, orgId, groupId)
	if err != nil {
		return models.Group{}, errors.Wrap(err, "get renamed group")
	}

	return group, nil
}

func (uc GroupUseCase) Delete(ctx context.Context, groupId uuid.UUID) error {
	if err := uc.enforceSecurity.ManageGroups(); err != nil {
		return err
	}
	if err := uc.enforceSecurity.KeepOwnAdmin(groupId); err != nil {
		return err
	}

	orgId := uc.enforceSecurity.OrgId()

	err := uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.groupRepository.LockGroup(ctx, tx, orgId, groupId); err != nil {
			return err
		}

		return uc.groupRepository.DeleteGroup(ctx, tx, orgId, groupId)
	})
	if err != nil {
		return errors.Wrap(err, "delete group")
	}

	return nil
}

func (uc GroupUseCase) ReplaceMembers(ctx context.Context, groupId uuid.UUID, memberIds []models.UserId) (models.Group, error) {
	if err := uc.enforceSecurity.ManageGroups(); err != nil {
		return models.Group{}, err
	}
	if !slices.Contains(memberIds, uc.credentials.ActorIdentity.UserId) {
		if err := uc.enforceSecurity.KeepOwnAdmin(groupId); err != nil {
			return models.Group{}, err
		}
	}

	orgId := uc.enforceSecurity.OrgId()
	memberIds = slices.Compact(slices.Sorted(slices.Values(memberIds)))

	err := uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.groupRepository.LockGroup(ctx, tx, orgId, groupId); err != nil {
			return err
		}

		group, err := uc.groupRepository.GetGroup(ctx, tx, orgId, groupId)
		if err != nil {
			return err
		}

		added := slices.DeleteFunc(slices.Clone(memberIds), func(id models.UserId) bool {
			return slices.Contains(group.MemberIds, id)
		})
		if len(added) == 0 {
			return uc.groupRepository.ReplaceGroupMembers(ctx, tx, groupId, memberIds)
		}

		// New members are granted all the role bindings of the group.
		if err := uc.enforceSecurity.GrantGroupRoleBindings(nil, group.RoleBindings); err != nil {
			return err
		}

		found, err := uc.groupRepository.FilterOrganizationTenantUsers(ctx, tx, orgId, added)
		if err != nil {
			return err
		}
		if len(found) != len(added) {
			return errors.Wrap(models.NotFoundError, "group members must be users of the organization's tenant")
		}

		return uc.groupRepository.ReplaceGroupMembers(ctx, tx, groupId, memberIds)
	})
	if err != nil {
		return models.Group{}, errors.Wrap(err, "replace group members")
	}

	return uc.Get(ctx, groupId)
}

func (uc GroupUseCase) ReplaceBindings(ctx context.Context, groupId uuid.UUID, bindings []models.RoleBinding) (models.Group, error) {
	if err := uc.enforceSecurity.ManageGroups(); err != nil {
		return models.Group{}, err
	}

	orgId := uc.enforceSecurity.OrgId()

	resolved, err := resolveRoleBindings(ctx, uc.executorFactory.NewExecutor(), uc.roleRepository,
		orgId, bindings, models.GetValidOrganizationGrantRoles())
	if err != nil {
		return models.Group{}, err
	}

	if !slices.ContainsFunc(resolved, func(binding models.RoleBinding) bool { return binding.Role == models.ADMIN }) {
		if err := uc.enforceSecurity.KeepOwnAdmin(groupId); err != nil {
			return models.Group{}, err
		}
	}

	err = uc.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := uc.groupRepository.LockGroup(ctx, tx, orgId, groupId); err != nil {
			return err
		}

		group, err := uc.groupRepository.GetGroup(ctx, tx, orgId, groupId)
		if err != nil {
			return err
		}

		if err := uc.enforceSecurity.GrantGroupRoleBindings(group.RoleBindings, resolved); err != nil {
			return err
		}

		return uc.groupRepository.ReplaceGroupRoleBindings(ctx, tx, orgId, groupId, resolved)
	})
	if err != nil {
		return models.Group{}, errors.Wrap(err, "replace group role bindings")
	}

	return uc.Get(ctx, groupId)
}

func validateGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.Wrap(models.BadParameterError, "group name cannot be empty")
	}

	return name, nil
}
