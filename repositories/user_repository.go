package repositories

import (
	"fmt"
	. "marble/marble-backend/models"
	"sync"
)

type UserRepository interface {
	UserByFirebaseUid(id string) *User
	UserByEmail(email string) *User
	UpdateFirebaseId(userId string, firebaseUid string) error
}

type UserRepositoryHardcoded struct {
	users sync.Map
}

func NewHardcodedUserRepository(hardcodedUsers []User) UserRepository {

	repo := UserRepositoryHardcoded{}

	for _, user := range hardcodedUsers {
		repo.users.Store(user.UserId, &user)
	}

	return &repo
}

func (repo *UserRepositoryHardcoded) findUser(predicate func(user User) bool) *User {
	var result *User
	repo.users.Range(func(_, u interface{}) bool {
		user := u.(*User)
		if predicate(*user) {
			result = user
			return false
		}
		return true
	})

	return result
}

func (repo *UserRepositoryHardcoded) findUserById(userId string) *User {
	user, ok := repo.users.Load(userId)
	if !ok {
		return nil
	}
	return user.(*User)
}

func (repo *UserRepositoryHardcoded) UserByFirebaseUid(firebaseUid string) *User {
	return repo.findUser(func(user User) bool {
		return user.FirebaseUid == firebaseUid
	})
}

func (repo *UserRepositoryHardcoded) UserByEmail(email string) *User {
	return repo.findUser(func(user User) bool {
		return user.Email == email
	})
}

func (repo *UserRepositoryHardcoded) UpdateFirebaseId(userId string, firebaseUid string) error {
	user := repo.findUserById(userId)
	if user == nil {
		return fmt.Errorf("User %s: %w", userId, NotFoundError)
	}
	user.FirebaseUid = firebaseUid
	return nil
}
