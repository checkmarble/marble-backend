package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type UserRepository struct {
	mock.Mock
}

func (r *UserRepository) CreateUser(ctx context.Context, tx repositories.Transaction_deprec, createUser models.CreateUser) (models.UserId, error) {
	args := r.Called(tx, createUser)
	return args.Get(0).(models.UserId), args.Error(1)
}

func (r *UserRepository) UpdateUser(ctx context.Context, tx repositories.Transaction_deprec, updateUser models.UpdateUser) error {
	args := r.Called(tx, updateUser)
	return args.Error(0)
}

func (r *UserRepository) DeleteUser(ctx context.Context, tx repositories.Transaction_deprec, userID models.UserId) error {
	args := r.Called(tx, userID)
	return args.Error(0)
}

func (r *UserRepository) DeleteUsersOfOrganization(ctx context.Context, tx repositories.Transaction_deprec, organizationId string) error {
	args := r.Called(tx, organizationId)
	return args.Error(0)
}

func (r *UserRepository) UserByID(ctx context.Context, tx repositories.Transaction_deprec, userId models.UserId) (models.User, error) {
	args := r.Called(tx, userId)
	return args.Get(0).(models.User), args.Error(1)
}

func (r *UserRepository) UsersOfOrganization(ctx context.Context, tx repositories.Transaction_deprec, organizationIDFilter string) ([]models.User, error) {
	args := r.Called(tx, organizationIDFilter)
	return args.Get(0).([]models.User), args.Error(1)
}

func (r *UserRepository) AllUsers(ctx context.Context, tx repositories.Transaction_deprec) ([]models.User, error) {
	args := r.Called(tx)
	return args.Get(0).([]models.User), args.Error(1)
}

func (r *UserRepository) UserByEmail(ctx context.Context, tx repositories.Transaction_deprec, email string) (*models.User, error) {
	args := r.Called(tx, email)
	return args.Get(0).(*models.User), args.Error(1)
}
