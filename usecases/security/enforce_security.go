package security

import (
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/clock"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"

	"github.com/cockroachdb/errors"
)

type EnforceSecurity interface {
	Permission(permission models.Permission) error
	ReadOrganization(organizationId uuid.UUID) error
	Permissions(permissions []models.Permission) error

	OrgId() uuid.UUID
	UserId() *string
	ApiKeyId() *string
}

type EnforceSecurityImpl struct {
	Credentials models.Credentials
	Clock       clock.Clock
}

func NewEnforceSecurity(credentials models.Credentials) *EnforceSecurityImpl {
	return &EnforceSecurityImpl{
		Credentials: credentials,
		Clock:       clock.New(),
	}
}

func (e *EnforceSecurityImpl) OrgId() uuid.UUID {
	return e.Credentials.OrganizationId
}

func (e *EnforceSecurityImpl) UserId() *string {
	if e.Credentials.ActorIdentity.UserId == "" {
		return nil
	}

	return utils.Ptr(string(e.Credentials.ActorIdentity.UserId))
}

func (e *EnforceSecurityImpl) ApiKeyId() *string {
	if e.Credentials.ActorIdentity.ApiKeyId == "" {
		return nil
	}

	return utils.Ptr(e.Credentials.ActorIdentity.ApiKeyId)
}

func (e *EnforceSecurityImpl) ReadOrganization(organizationId uuid.UUID) error {
	return utils.EnforceOrganizationAccess(e.Credentials, organizationId)
}

func (e *EnforceSecurityImpl) Permissions(permissions []models.Permission) error {
	for _, p := range permissions {
		if err := e.Permission(p); err != nil {
			return err
		}
	}
	return nil
}

func (e *EnforceSecurityImpl) Permission(permission models.Permission) error {
	allowed := false

	if len(e.Credentials.RoleBindings) > 0 {
		now := time.Now()

		if e.Clock != nil {
			now = e.Clock.Now()
		}

		for _, binding := range e.Credentials.RoleBindings {
			if binding.IsActive(now) && slices.Contains(binding.Permissions, permission) {
				allowed = true
				break
			}
		}
	} else {
		allowed = e.Credentials.HasPermission(permission)
	}

	if !allowed {
		return errors.Wrap(models.ForbiddenError, "missing permission "+string(permission))
	}

	return nil
}
