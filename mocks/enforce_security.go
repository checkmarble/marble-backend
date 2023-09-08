package mocks

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/mock"
)

type EnforceSecurity struct {
	mock.Mock
}

func (e *EnforceSecurity) ReadOrganization(organizationId string) error {
	args := e.Called(organizationId)
	return args.Error(0)
}

func (e *EnforceSecurity) Permission(permission models.Permission) error {
	args := e.Called(permission)
	return args.Error(0)
}
