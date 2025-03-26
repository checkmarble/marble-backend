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

func (r *UserRepository) CreateUser(ctx context.Context, exec repositories.Executor, createUser models.CreateUser) (string, error) {
	args := r.Called(ctx, exec, createUser)
	return args.Get(0).(string), args.Error(1)
}

func (r *UserRepository) UpdateUser(ctx context.Context, exec repositories.Executor, updateUser models.UpdateUser) error {
	args := r.Called(ctx, exec, updateUser)
	return args.Error(0)
}

func (r *UserRepository) DeleteUser(ctx context.Context, exec repositories.Executor, userID models.UserId) error {
	args := r.Called(ctx, exec, userID)
	return args.Error(0)
}

func (r *UserRepository) DeleteUsersOfOrganization(ctx context.Context, exec repositories.Executor, organizationId string) error {
	args := r.Called(ctx, exec, organizationId)
	return args.Error(0)
}

func (r *UserRepository) UserById(ctx context.Context, exec repositories.Executor, userId string) (models.User, error) {
	args := r.Called(ctx, exec, userId)
	return args.Get(0).(models.User), args.Error(1)
}

func (r *UserRepository) ListUsers(ctx context.Context, exec repositories.Executor, filterOrganisationId *string) ([]models.User, error) {
	args := r.Called(ctx, exec, filterOrganisationId)
	return args.Get(0).([]models.User), args.Error(1)
}

func (r *UserRepository) UserByEmail(ctx context.Context, exec repositories.Executor, email string) (*models.User, error) {
	args := r.Called(ctx, exec, email)
	return args.Get(0).(*models.User), args.Error(1)
}

func (r *UserRepository) HasUsers(ctx context.Context, exec repositories.Executor) (bool, error) {
	args := r.Called(ctx, exec)
	return args.Bool(0), args.Error(1)
}
