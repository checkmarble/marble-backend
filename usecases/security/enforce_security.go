package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
)

type EnforceSecurity interface {
	Permission(permission models.Permission) error
	ReadOrganization(organizationId string) error
	Permissions(permissions []models.Permission) error
}

type EnforceSecurityImpl struct {
	Credentials models.Credentials
}

func NewEnforceSecurity(credentials models.Credentials) *EnforceSecurityImpl {
	return &EnforceSecurityImpl{
		Credentials: credentials,
	}
}

func (e *EnforceSecurityImpl) ReadOrganization(organizationId string) error {
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
	permissionStr, err := permission.String()
	if err != nil {
		return errors.Wrap(err, "failed to adapt permission to string")
	}

	if !e.Credentials.Role.HasPermission(permission) {
		return errors.Wrap(models.ForbiddenError, "missing permission "+permissionStr)
	}
	return nil
}
