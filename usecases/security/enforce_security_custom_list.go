package security

import (
	"errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type EnforceSecurityCustomList interface {
	EnforceSecurity
	ReadCustomList(customList models.CustomList) error
	ModifyCustomList(customList models.CustomList) error
	CreateCustomList() error
}

type EnforceSecurityCustomListImpl struct {
	EnforceSecurity
	Credentials models.Credentials
}

func (e *EnforceSecurityCustomListImpl) ReadCustomList(customList models.CustomList) error {
	orgId, _ := uuid.Parse(customList.OrganizationId)
	return errors.Join(
		e.Permission(models.CUSTOM_LISTS_READ),
		e.ReadOrganization(orgId),
	)
}

func (e *EnforceSecurityCustomListImpl) CreateCustomList() error {
	return errors.Join(
		e.Permission(models.CUSTOM_LISTS_EDIT),
	)
}

func (e *EnforceSecurityCustomListImpl) ModifyCustomList(customList models.CustomList) error {
	orgId, _ := uuid.Parse(customList.OrganizationId)
	return errors.Join(
		e.Permission(models.CUSTOM_LISTS_EDIT),
		e.ReadOrganization(orgId),
	)
}
