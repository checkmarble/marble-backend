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
	credentials            models.Credentials
	enforceUserSecurity    security.EnforceSecurityUser
	executorFactory        executor_factory.ExecutorFactory
	transactionFactory     executor_factory.TransactionFactory
	userRepository         repositories.UserRepository
	grantRepository        repositories.GrantRepository
	organizationRepository repositories.OrganizationRepository
	firebaseAdmin          idp.Adminer
}

func (usecase *UserUseCase) authorizeTenantAdministration(tenantID uuid.UUID) error {
	c := usecase.credentials
	if tenantID == uuid.Nil {
		return models.BadParameterError
	}
	if c.HasRole(models.MARBLE_ADMIN) {
		return nil
	}
	if c.HasRole(models.TENANT_ADMIN) && c.TenantId == tenantID && (c.AuthTenantId == uuid.Nil || c.AuthTenantId == tenantID) {
		return nil
	}
	return models.ForbiddenError
}

func (usecase *UserUseCase) ListUsersByTenant(ctx context.Context, tenantID uuid.UUID) ([]models.User, error) {
	if err := usecase.authorizeTenantAdministration(tenantID); err != nil {
		return nil, err
	}
	return usecase.userRepository.ListUsersByTenant(ctx, usecase.executorFactory.NewExecutor(), tenantID)
}

func (usecase *UserUseCase) SetTenantAdministrator(ctx context.Context, tenantID uuid.UUID, userID string, enabled bool) error {
	if err := usecase.authorizeTenantAdministration(tenantID); err != nil {
		return err
	}
	if userID == string(usecase.credentials.ActorIdentity.UserId) {
		return errors.Wrap(models.ForbiddenError, "cannot change your own tenant administrator grant")
	}
	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return usecase.grantRepository.SetTenantAdministrator(ctx, tx, tenantID, userID, enabled)
	})
}

func (usecase *UserUseCase) AddUser(ctx context.Context, createUser models.CreateUser) (models.User, error) {
	if createUser.OrganizationId == uuid.Nil && createUser.Role != models.NO_ROLE && createUser.Role != models.MARBLE_ADMIN {
		return models.User{}, errors.Wrap(models.BadParameterError, "tenant identities are created without a role; assign grants separately")
	}
	if createUser.OrganizationId != uuid.Nil && !slices.Contains(models.GetValidOrganizationGrantRoles(), createUser.Role) && createUser.Role != models.MARBLE_ADMIN {
		return models.User{}, models.BadParameterError
	}
	// Only marble admins can exist outside of an organization
	if createUser.OrganizationId == uuid.Nil && createUser.TenantId == uuid.Nil && createUser.Role != models.MARBLE_ADMIN {
		return models.User{}, errors.Wrap(models.BadParameterError,
			"organization_id is required unless the role is MARBLE_ADMIN")
	}

	var org models.Organization
	if createUser.OrganizationId != uuid.Nil {
		var err error
		org, err = usecase.organizationRepository.GetOrganizationById(ctx, usecase.executorFactory.NewExecutor(), createUser.OrganizationId)
		if err != nil {
			return models.User{}, errors.Wrap(err, "GetOrganizationById error")
		}
		if createUser.TenantId != uuid.Nil && createUser.TenantId != org.TenantId {
			return models.User{}, models.ForbiddenError
		}
		createUser.TenantId = org.TenantId
	}

	if err := usecase.enforceUserSecurity.CreateUser(createUser); err != nil {
		return models.User{}, err
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
		"email":      createdUser.Email,
		"tenant_id":  createdUser.TenantId,
		"first_name": createdUser.FirstName,
		"last_name":  createdUser.LastName,
	})
	if createUser.OrganizationId != uuid.Nil {
		tracking.Group(ctx, createdUser.UserId, createUser.OrganizationId, map[string]any{
			"name": org.Name,
		})
	}
	tracking.TrackEvent(ctx, models.AnalyticsUserCreated, map[string]interface{}{
		"user_id":   createdUser.UserId,
		"email":     createdUser.Email,
		"tenant_id": createdUser.TenantId,
	})

	return createdUser, nil
}

func (usecase *UserUseCase) UpdateUser(ctx context.Context, updateUser models.UpdateUser) (models.User, error) {
	if updateUser.OrganizationId == uuid.Nil {
		if creds, ok := utils.CredentialsFromCtx(ctx); ok {
			updateUser.OrganizationId = creds.OrganizationId
		}
	}

	updatedUser, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.User, error) {
			user, err := usecase.userRepository.UserById(ctx, tx, updateUser.UserId)
			if err != nil {
				return models.User{}, err
			}
			// Profile forms may echo the compatibility role without selecting an organization.
			if updateUser.Role != nil && updateUser.OrganizationId == uuid.Nil && *updateUser.Role == user.Role {
				updateUser.Role = nil
			}
			if updateUser.Role != nil && !slices.Contains(models.GetValidUserRoles(), *updateUser.Role) {
				return models.User{}, errors.Wrap(models.BadParameterError, "Invalid role received")
			}
			if updateUser.Email != nil && *updateUser.Email == user.Email {
				updateUser.Email = nil
			}
			if updateUser.FirstName != nil && *updateUser.FirstName == user.FirstName {
				updateUser.FirstName = nil
			}
			if updateUser.LastName != nil && *updateUser.LastName == user.LastName {
				updateUser.LastName = nil
			}
			if err := usecase.enforceUserSecurity.UpdateUser(user, updateUser); err != nil {
				return models.User{}, err
			}
			if updateUser.Role != nil && *updateUser.Role != models.MARBLE_ADMIN {
				org, err := usecase.organizationRepository.GetOrganizationById(ctx, tx, updateUser.OrganizationId)
				if err != nil {
					return models.User{}, err
				}
				if user.TenantId != org.TenantId {
					return models.User{}, models.ForbiddenError
				}
				if err := usecase.enforceUserSecurity.ManageOrganizationGrant(org.Id, user); err != nil {
					return models.User{}, err
				}
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
		"email":      updatedUser.Email,
		"tenant_id":  updatedUser.TenantId,
		"first_name": updatedUser.FirstName,
		"last_name":  updatedUser.LastName,
	})
	tracking.TrackEvent(ctx, models.AnalyticsUserUpdated, map[string]interface{}{
		"user_id":   updatedUser.UserId,
		"email":     updatedUser.Email,
		"tenant_id": updatedUser.TenantId,
	})

	return updatedUser, nil
}

func (usecase *UserUseCase) DeleteUser(ctx context.Context, userId, currentUserId string) error {
	if userId == currentUserId {
		return errors.Wrap(models.ForbiddenError, "cannot delete yourself")
	}
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		user, err := usecase.userRepository.UserById(ctx, tx, userId)
		if err != nil {
			return err
		}
		if err := usecase.enforceUserSecurity.DeleteUser(user); err != nil {
			return err
		}
		return usecase.userRepository.DeleteUser(ctx, tx, models.UserId(userId))
	})
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

func (usecase *UserUseCase) ListUserGrants(ctx context.Context, userID string) ([]models.Grant, error) {
	if err := usecase.enforceUserSecurity.ListUserGrants(); err != nil {
		return nil, err
	}
	exec := usecase.executorFactory.NewExecutor()
	user, err := usecase.userRepository.UserById(ctx, exec, userID)
	if err != nil {
		return nil, err
	}
	if err := usecase.enforceUserSecurity.ReadUser(user); err != nil {
		return nil, err
	}
	grantsByUser, err := usecase.grantRepository.ListOrganizationGrantsByUser(ctx, exec, []string{userID})
	if err != nil {
		return nil, err
	}
	return usecase.visibleOrganizationGrants(grantsByUser[userID]), nil
}

// ListGrantsOfUsers returns the organization grants of the given users, keyed by user id.
func (usecase *UserUseCase) ListGrantsOfUsers(ctx context.Context, users []models.User) (map[string][]models.Grant, error) {
	if err := usecase.enforceUserSecurity.ListUserGrants(); err != nil {
		return nil, err
	}
	for _, user := range users {
		if err := usecase.enforceUserSecurity.ReadUser(user); err != nil {
			return nil, err
		}
	}
	userIDs := pure_utils.Map(users, func(user models.User) string { return string(user.UserId) })
	grants, err := usecase.grantRepository.ListOrganizationGrantsByUser(ctx, usecase.executorFactory.NewExecutor(), userIDs)
	if err != nil {
		return nil, err
	}
	for id, userGrants := range grants {
		grants[id] = usecase.visibleOrganizationGrants(userGrants)
	}
	return grants, nil
}

func (usecase *UserUseCase) visibleOrganizationGrants(grants []models.Grant) []models.Grant {
	if usecase.credentials.HasRole(models.MARBLE_ADMIN) {
		return grants
	}
	return slices.DeleteFunc(grants, func(grant models.Grant) bool {
		return grant.TenantId != usecase.credentials.TenantId || grant.TenantId == uuid.Nil
	})
}

func (usecase *UserUseCase) ReplaceOrganizationGrant(ctx context.Context, userID string, tenantID, organizationID uuid.UUID, role models.Role) error {
	if !slices.Contains(models.GetValidOrganizationGrantRoles(), role) {
		return errors.Wrap(models.BadParameterError, "invalid organization grant role")
	}
	if role != models.ADMIN && usecase.isLocalAdministratorSelf(userID) {
		return models.ForbiddenError
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		targetOrganization, err := usecase.organizationRepository.GetOrganizationById(ctx, tx, organizationID)
		if err != nil {
			return err
		}
		if targetOrganization.TenantId != tenantID {
			return errors.Wrap(models.NotFoundError, "organization does not belong to tenant")
		}
		user, err := usecase.userRepository.UserById(ctx, tx, userID)
		if err != nil {
			return err
		}
		if err := usecase.enforceUserSecurity.ManageOrganizationGrant(organizationID, user); err != nil {
			return err
		}
		if err := usecase.ensureUserBelongsToOrganizationTenant(ctx, tx, user, targetOrganization); err != nil {
			return err
		}
		return usecase.grantRepository.ReplaceOrganizationGrant(ctx, tx, userID, organizationID, role)
	})
}

func (usecase *UserUseCase) RevokeOrganizationGrant(ctx context.Context, userID string, tenantID, organizationID uuid.UUID) error {
	if usecase.isLocalAdministratorSelf(userID) {
		return models.ForbiddenError
	}
	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		targetOrganization, err := usecase.organizationRepository.GetOrganizationById(ctx, tx, organizationID)
		if err != nil {
			return err
		}
		if targetOrganization.TenantId != tenantID {
			return errors.Wrap(models.NotFoundError, "organization does not belong to tenant")
		}
		user, err := usecase.userRepository.UserById(ctx, tx, userID)
		if err != nil {
			return err
		}
		if err := usecase.enforceUserSecurity.ManageOrganizationGrant(organizationID, user); err != nil {
			return err
		}
		if err := usecase.ensureUserBelongsToOrganizationTenant(ctx, tx, user, targetOrganization); err != nil {
			return err
		}
		return usecase.grantRepository.RevokeOrganizationGrant(ctx, tx, userID, organizationID)
	})
}

func (usecase *UserUseCase) isLocalAdministratorSelf(userID string) bool {
	c := usecase.credentials
	return userID == string(c.ActorIdentity.UserId) && c.HasRole(models.ADMIN) && !c.HasRole(models.TENANT_ADMIN) && !c.HasRole(models.MARBLE_ADMIN)
}

func (usecase *UserUseCase) ensureUserBelongsToOrganizationTenant(ctx context.Context, exec repositories.Executor, user models.User, targetOrganization models.Organization) error {
	if user.Role == models.MARBLE_ADMIN {
		return nil
	}
	if user.TenantId != targetOrganization.TenantId {
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
