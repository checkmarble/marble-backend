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

func (r *UserRepository) CreateUser(ctx context.Context, exec repositories.Executor, createUser models.CreateUser) (models.UserId, error) {
	args := r.Called(exec, createUser)
	return args.Get(0).(models.UserId), args.Error(1)
}

func (r *UserRepository) UpdateUser(ctx context.Context, exec repositories.Executor, updateUser models.UpdateUser) error {
	args := r.Called(exec, updateUser)
	return args.Error(0)
}

func (r *UserRepository) DeleteUser(ctx context.Context, exec repositories.Executor, userID models.UserId) error {
	args := r.Called(exec, userID)
	return args.Error(0)
}

func (r *UserRepository) DeleteUsersOfOrganization(ctx context.Context, exec repositories.Executor, organizationId string) error {
	args := r.Called(exec, organizationId)
	return args.Error(0)
}

func (r *UserRepository) UserByID(ctx context.Context, exec repositories.Executor, userId models.UserId) (models.User, error) {
	args := r.Called(exec, userId)
	return args.Get(0).(models.User), args.Error(1)
}

func (r *UserRepository) UsersOfOrganization(ctx context.Context, exec repositories.Executor, organizationIDFilter string) ([]models.User, error) {
	args := r.Called(exec, organizationIDFilter)
	return args.Get(0).([]models.User), args.Error(1)
}

func (r *UserRepository) AllUsers(ctx context.Context, exec repositories.Executor) ([]models.User, error) {
	args := r.Called(exec)
	return args.Get(0).([]models.User), args.Error(1)
}

func (r *UserRepository) UserByEmail(ctx context.Context, exec repositories.Executor, email string) (*models.User, error) {
	args := r.Called(exec, email)
	return args.Get(0).(*models.User), args.Error(1)
}
