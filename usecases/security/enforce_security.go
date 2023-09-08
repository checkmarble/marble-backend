package security

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
)
//go:generate mockgen -destination=./enforce_security_mock.go -package=security marble/marble-backend/usecases/security EnforceSecurity
type EnforceSecurity interface {
	Permission(permission models.Permission) error
	ReadOrganization(organizationId string) error
}

type EnforceSecurityImpl struct {
	Credentials models.Credentials
}

func (e *EnforceSecurityImpl) ReadOrganization(organizationId string) error {
	return utils.EnforceOrganizationAccess(e.Credentials, organizationId)
}

func (e *EnforceSecurityImpl) Permission(permission models.Permission) error {
	if !e.Credentials.Role.HasPermission(permission) {
		return errors.Wrap(models.ForbiddenError, "missing permission %s"+permission.String())
	}
	return nil
}
