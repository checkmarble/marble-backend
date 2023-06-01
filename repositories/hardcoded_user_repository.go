package repositories

import (
	"marble/marble-backend/models"
	"sync"
)

type HardcodedUsersRepository struct {
	users sync.Map
}

func (repo *HardcodedUsersRepository) findUser(predicate func(user models.User) bool) *models.User {
	var result *models.User
	repo.users.Range(func(_, u any) bool {
		user := u.(models.User)
		if predicate(user) {
			result = &user
			return false
		}
		return true
	})

	return result
}

func NewHardcodedUserRepository(hardcodedUsers []models.User) *HardcodedUsersRepository {

	repo := &HardcodedUsersRepository{}

	for _, user := range hardcodedUsers {
		repo.users.Store(user.UserId, user)
	}

	return repo
}

func (repo *HardcodedUsersRepository) UserByEmail(email string) *models.User {
	return repo.findUser(func(user models.User) bool {
		return user.Email == email
	})
}
