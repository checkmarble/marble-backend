package usecases

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/repositories/idp"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"

	"github.com/cockroachdb/errors"
)

type UserUseCase struct {
	enforceUserSecurity    security.EnforceSecurityUser
	executorFactory        executor_factory.ExecutorFactory
	transactionFactory     executor_factory.TransactionFactory
	userRepository         repositories.UserRepository
	grantRepository        repositories.GrantRepository
	organizationRepository repositories.OrganizationRepository
	firebaseAdmin          idp.Adminer
}

func (usecase *UserUseCase) AddUser(ctx context.Context, createUser models.CreateUser) (models.User, error) {
	if len(createUser.RoleBindings) > 0 {
		bindings, err := usecase.resolveUserRoleBindings(ctx, createUser.OrganizationId, createUser.RoleBindings)
		if err != nil {
			return models.User{}, err
		}
		createUser.RoleBindings = bindings
	}

	if err := usecase.enforceUserSecurity.CreateUser(createUser); err != nil {
		return models.User{}, err
	}

	org, err := usecase.organizationRepository.GetOrganizationById(ctx, usecase.executorFactory.NewExecutor(), createUser.OrganizationId)
	if err != nil {
		return models.User{}, errors.Wrap(err, "GetOrganizationById error")
	}

	createdUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.User, error) {
			// cleanup spaces
			createUser.Email = strings.TrimSpace(createUser.Email)
			// lowercase email to maintain uniqueness
			createUser.Email = strings.ToLower(createUser.Email)

			createdUserUuid, err := usecase.userRepository.CreateUser(ctx, tx, createUser)
			if repositories.IsUniqueViolationError(err) {
				return models.User{}, models.ConflictError
			}
			if err != nil {
				return models.User{}, err
			}

			if usecase.firebaseAdmin != nil {
				if err := usecase.firebaseAdmin.CreateUser(ctx, createUser.Email,
					fmt.Sprintf("%s %s", createUser.FirstName, createUser.LastName)); err != nil {
					return models.User{}, errors.Wrap(err, "could not create Firebase user")
				}
			}

			return usecase.userRepository.UserById(ctx, tx, createdUserUuid)
		},
	)
	if err != nil {
		return models.User{}, err
	}
	tracking.Identify(ctx, createdUser.UserId, map[string]interface{}{
		"email":           createdUser.Email,
		"organization_id": createdUser.OrganizationId,
		"first_name":      createdUser.FirstName,
		"last_name":       createdUser.LastName,
	})
	tracking.Group(ctx, createdUser.UserId, createdUser.OrganizationId, map[string]any{
		"name": org.Name,
	})
	tracking.TrackEvent(ctx, models.AnalyticsUserCreated, map[string]interface{}{
		"user_id":         createdUser.UserId,
		"email":           createdUser.Email,
		"organization_id": createdUser.OrganizationId,
	})

	return createdUser, nil
}

func (usecase *UserUseCase) UpdateUser(ctx context.Context, updateUser models.UpdateUser) (models.User, error) {
	if updateUser.RoleBindings != nil {
		user, err := usecase.userRepository.UserById(ctx, usecase.executorFactory.NewExecutor(), updateUser.UserId)
		if err != nil {
			return models.User{}, err
		}
		bindings, err := usecase.resolveUserRoleBindings(ctx, user.OrganizationId, *updateUser.RoleBindings)
		if err != nil {
			return models.User{}, err
		}
		updateUser.RoleBindings = &bindings
	}

	updatedUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.User, error) {
			user, err := usecase.userRepository.UserById(ctx, tx, updateUser.UserId)
			if err != nil {
				return models.User{}, err
			}
			if err := usecase.enforceUserSecurity.UpdateUser(user, updateUser); err != nil {
				return models.User{}, err
			}
			if err := usecase.userRepository.UpdateUser(ctx, tx, updateUser); err != nil {
				return models.User{}, err
			}
			return usecase.userRepository.UserById(ctx, tx, updateUser.UserId)
		},
	)
	if err != nil {
		return models.User{}, err
	}
	tracking.Identify(ctx, updatedUser.UserId, map[string]interface{}{
		"email":           updatedUser.Email,
		"organization_id": updatedUser.OrganizationId,
		"first_name":      updatedUser.FirstName,
		"last_name":       updatedUser.LastName,
	})
	tracking.TrackEvent(ctx, models.AnalyticsUserUpdated, map[string]interface{}{
		"user_id":         updatedUser.UserId,
		"email":           updatedUser.Email,
		"organization_id": updatedUser.OrganizationId,
	})

	return updatedUser, nil
}

func (usecase *UserUseCase) resolveUserRoleBindings(
	ctx context.Context,
	orgId uuid.UUID,
	bindings []models.RoleBinding,
) ([]models.RoleBinding, error) {
	resolved := append([]models.RoleBinding(nil), bindings...)

	for idx := range resolved {
		binding := &resolved[idx]
		if binding.Role == "" {
			return nil, errors.Wrap(models.BadParameterError, "role binding must reference a role")
		}

		if !slices.Contains(models.GetValidUserRoles(), binding.Role) {
			return nil, errors.Wrap(models.BadParameterError, "invalid role")
		}
		binding.Permissions = binding.Role.Permissions()
	}

	return resolved, nil
}

func (usecase *UserUseCase) DeleteUser(ctx context.Context, userId, currentUserId string) error {
	if userId == currentUserId {
		return errors.Wrap(models.ForbiddenError, "cannot delete yourself")
	}
	exec := usecase.executorFactory.NewExecutor()
	user, err := usecase.userRepository.UserById(ctx, exec, userId)
	if err != nil {
		return err
	}
	if err := usecase.enforceUserSecurity.DeleteUser(user); err != nil {
		return err
	}
	err = usecase.userRepository.DeleteUser(ctx, exec, models.UserId(userId))
	if err != nil {
		return err
	}
	tracking.TrackEvent(ctx, models.AnalyticsUserDeleted, map[string]interface{}{
		"user_id": userId,
	})

	return nil
}

func (usecase *UserUseCase) ListUsers(ctx context.Context, organisationId *uuid.UUID, withTfa bool) ([]models.User, error) {
	if err := usecase.enforceUserSecurity.ListUsers(organisationId); err != nil {
		return nil, err
	}

	exec := usecase.executorFactory.NewExecutor()
	users, err := usecase.userRepository.ListUsers(ctx, exec, organisationId)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if err = usecase.enforceUserSecurity.ReadUser(u); err != nil {
			return nil, err
		}
	}

	if withTfa {
		usecase.enrichWithTfa(ctx, users)
	}

	return users, nil
}

func (usecase *UserUseCase) ListTenantUsers(ctx context.Context, organizationID uuid.UUID, access string) ([]models.OrganizationUserGrant, error) {
	if err := usecase.enforceUserSecurity.ListTenantUsers(organizationID); err != nil {
		return nil, err
	}
	exec := usecase.executorFactory.NewExecutor()
	organization, err := usecase.organizationRepository.GetOrganizationById(ctx, exec, organizationID)
	if err != nil {
		return nil, err
	}

	switch access {
	case "direct":
		return usecase.grantRepository.ListTenantUsersWithDirectOrganizationGrant(ctx, exec, organization.TenantId, organizationID)
	case "missing":
		return usecase.grantRepository.ListTenantUsersWithoutOrganizationAccess(ctx, exec, organization.TenantId, organizationID)
	default:
		return nil, errors.Wrap(models.BadParameterError, "invalid tenant_access")
	}
}

// ReplaceOrganizationGrant replaces the role bindings of a user in an
// organization of its tenant, which may not be its home organization. An empty
// list of bindings removes the user's access to the organization.
func (usecase *UserUseCase) ReplaceOrganizationGrant(
	ctx context.Context,
	userID string,
	organizationID uuid.UUID,
	bindings []models.RoleBinding,
) error {
	resolved, err := usecase.resolveOrganizationRoleBindings(ctx, organizationID, bindings)
	if err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		user, err := usecase.userRepository.UserById(ctx, tx, userID)
		if err != nil {
			return err
		}
		if err := usecase.enforceUserSecurity.ManageOrganizationGrant(organizationID, user); err != nil {
			return err
		}
		if err := usecase.ensureUserBelongsToOrganizationTenant(ctx, tx, user, organizationID); err != nil {
			return err
		}
		return usecase.userRepository.ReplaceUserOrganizationRoleBindings(ctx, tx, organizationID, userID, resolved)
	})
}

// RevokeOrganizationGrant removes all role bindings of a user in an
// organization of its tenant.
func (usecase *UserUseCase) RevokeOrganizationGrant(ctx context.Context, userID string, organizationID uuid.UUID) error {
	return usecase.ReplaceOrganizationGrant(ctx, userID, organizationID, []models.RoleBinding{})
}

// resolveOrganizationRoleBindings resolves bindings for an organization a user
// may not belong to, where platform roles cannot be bound.
func (usecase *UserUseCase) resolveOrganizationRoleBindings(
	ctx context.Context,
	organizationID uuid.UUID,
	bindings []models.RoleBinding,
) ([]models.RoleBinding, error) {
	resolved, err := usecase.resolveUserRoleBindings(ctx, organizationID, bindings)
	if err != nil {
		return nil, err
	}

	for _, binding := range resolved {
		if !slices.Contains(models.GetValidOrganizationGrantRoles(), binding.Role) {
			return nil, errors.Wrapf(models.BadParameterError, "role %s cannot be granted in an organization", binding.Role)
		}
	}

	return resolved, nil
}

func (usecase *UserUseCase) ensureUserBelongsToOrganizationTenant(ctx context.Context, exec repositories.Executor, user models.User, organizationID uuid.UUID) error {
	targetOrganization, err := usecase.organizationRepository.GetOrganizationById(ctx, exec, organizationID)
	if err != nil {
		return err
	}
	// Marble admins do not belong to a tenant.
	if slices.Contains(models.RoleNames(user.RoleBindings), models.MARBLE_ADMIN) {
		return nil
	}
	if user.OrganizationId == uuid.Nil {
		return errors.Wrap(models.NotFoundError, "user has no organization")
	}
	homeOrganization, err := usecase.organizationRepository.GetOrganizationById(ctx, exec, user.OrganizationId)
	if err != nil {
		return err
	}
	if homeOrganization.TenantId != targetOrganization.TenantId {
		return errors.Wrap(models.NotFoundError, "user does not belong to organization tenant")
	}
	return nil
}

// enrichWithTfa populates the TfaEnabled field on each user from the identity
// provider. It is a no-op when no Firebase admin client is configured (e.g.
// OIDC deployments), where MFA enrollment is not tracked here. A provider
// failure is logged and leaves TfaEnabled nil rather than failing the listing.
func (usecase *UserUseCase) enrichWithTfa(ctx context.Context, users []models.User) {
	if usecase.firebaseAdmin == nil {
		return
	}

	enrollment, err := usecase.firebaseAdmin.ListMfaEnrollment(ctx, pure_utils.Map(users, func(u models.User) string { return u.Email }))
	if err != nil {
		utils.LoggerFromContext(ctx).WarnContext(ctx,
			"could not fetch MFA enrollment from identity provider, omitting tfa_enabled",
			"error", err.Error())
		return
	}

	for i := range users {
		enabled, found := enrollment[strings.ToLower(users[i].Email)]
		if found {
			users[i].TfaEnabled = &enabled
		}
	}
}

func (usecase *UserUseCase) GetUser(ctx context.Context, userID string) (models.User, error) {
	user, err := usecase.userRepository.UserById(ctx, usecase.executorFactory.NewExecutor(), userID)
	if err != nil {
		return models.User{}, err
	}

	if err := usecase.enforceUserSecurity.ReadUser(user); err != nil {
		return models.User{}, err
	}

	return user, nil
}

func (usecase *UserUseCase) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	user, err := usecase.userRepository.UserByEmail(ctx, usecase.executorFactory.NewExecutor(), email)
	if err != nil {
		return models.User{}, err
	}
	if user == nil {
		return models.User{}, errors.Wrap(models.NotFoundError, "no user found with this email")
	}

	if err := usecase.enforceUserSecurity.ReadUser(*user); err != nil {
		return models.User{}, err
	}

	return *user, nil
}
