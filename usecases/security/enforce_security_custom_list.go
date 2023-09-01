package security

import (
	"errors"
	"marble/marble-backend/models"
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
	return errors.Join(
		e.Permission(models.CUSTOM_LISTS_READ),
		e.ReadOrganization(customList.OrganizationId),
	)
}

func (e *EnforceSecurityCustomListImpl) CreateCustomList() error {
	return errors.Join(
		e.Permission(models.CUSTOM_LISTS_CREATE),
	)
}

func (e *EnforceSecurityCustomListImpl) ModifyCustomList(customList models.CustomList) error {
	return errors.Join(
		e.Permission(models.CUSTOM_LISTS_CREATE),
		e.ReadOrganization(customList.OrganizationId),
	)
}
