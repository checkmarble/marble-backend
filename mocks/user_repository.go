package mocks

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type UserRepository struct {
	mock.Mock
}

func (r *UserRepository) CreateUser(tx repositories.Transaction, createUser models.CreateUser) (models.UserId, error) {
	args := r.Called(tx, createUser)
	return args.Get(0).(models.UserId), args.Error(1)
}

func (r *UserRepository) DeleteUser(tx repositories.Transaction, userID models.UserId) error {
	args := r.Called(tx, userID)
	return args.Error(0)
}

func (r *UserRepository) DeleteUsersOfOrganization(tx repositories.Transaction, organizationId string) error {
	args := r.Called(tx, organizationId)
	return args.Error(0)
}

func (r *UserRepository) UserByID(tx repositories.Transaction, userId models.UserId) (models.User, error) {
	args := r.Called(tx, userId)
	return args.Get(0).(models.User), args.Error(1)
}

func (r *UserRepository) UsersOfOrganization(tx repositories.Transaction, organizationIDFilter string) ([]models.User, error) {
	args := r.Called(tx, organizationIDFilter)
	return args.Get(0).([]models.User), args.Error(1)
}

func (r *UserRepository) AllUsers(tx repositories.Transaction) ([]models.User, error) {
	args := r.Called(tx)
	return args.Get(0).([]models.User), args.Error(1)
}

func (r *UserRepository) UserByFirebaseUid(tx repositories.Transaction, firebaseUid string) (*models.User, error) {
	args := r.Called(tx, firebaseUid)
	return args.Get(0).(*models.User), args.Error(1)
}

func (r *UserRepository) UserByEmail(tx repositories.Transaction, email string) (*models.User, error) {
	args := r.Called(tx, email)
	return args.Get(0).(*models.User), args.Error(1)
}

func (r *UserRepository) UpdateFirebaseId(tx repositories.Transaction, userId models.UserId, firebaseUid string) error {
	args := r.Called(tx, userId, firebaseUid)
	return args.Error(0)
}
