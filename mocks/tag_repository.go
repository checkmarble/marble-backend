package mocks

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type TagUseCaseRepository struct {
	mock.Mock
}

func (_m *TagUseCaseRepository) ListOrganizationTags(exec repositories.Executor, organizationId string) ([]models.Tag, error) {
	args := _m.Called(exec, organizationId)
	return args.Get(0).([]models.Tag), args.Error(1)
}

func (_m *TagUseCaseRepository) CreateTag(exec repositories.Executor, attributes models.CreateTagAttributes, newTagId string) error {
	args := _m.Called(exec, attributes, newTagId)
	return args.Error(0)
}

func (_m *TagUseCaseRepository) UpdateTag(exec repositories.Executor, attributes models.UpdateTagAttributes) error {
	args := _m.Called(exec, attributes)
	return args.Error(0)
}

func (_m *TagUseCaseRepository) GetTagById(exec repositories.Executor, tagId string) (models.Tag, error) {
	args := _m.Called(exec, tagId)
	return args.Get(0).(models.Tag), args.Error(1)
}

func (_m *TagUseCaseRepository) SoftDeleteTag(exec repositories.Executor, tagId string) error {
	args := _m.Called(exec, tagId)
	return args.Error(0)
}
