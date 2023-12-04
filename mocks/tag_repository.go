package mocks

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type TagUseCaseRepository struct {
	mock.Mock
}

func (_m *TagUseCaseRepository) ListOrganizationTags(tx repositories.Transaction, organizationId string) ([]models.Tag, error) {
	args := _m.Called(tx, organizationId)
	return args.Get(0).([]models.Tag), args.Error(1)
}

func (_m *TagUseCaseRepository) CreateTag(tx repositories.Transaction, attributes models.CreateTagAttributes, newTagId string) error {
	args := _m.Called(tx, attributes, newTagId)
	return args.Error(0)
}

func (_m *TagUseCaseRepository) UpdateTag(tx repositories.Transaction, attributes models.UpdateTagAttributes) error {
	args := _m.Called(tx, attributes)
	return args.Error(0)
}

func (_m *TagUseCaseRepository) GetTagById(tx repositories.Transaction, tagId string) (models.Tag, error) {
	args := _m.Called(tx, tagId)
	return args.Get(0).(models.Tag), args.Error(1)
}

func (_m *TagUseCaseRepository) SoftDeleteTag(tx repositories.Transaction, tagId string) error {
	args := _m.Called(tx, tagId)
	return args.Error(0)
}
