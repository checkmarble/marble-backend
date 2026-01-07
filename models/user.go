package models

import (
	"time"

	"github.com/google/uuid"
)

type UserId string

type User struct {
	UserId         UserId
	Email          string
	Role           Role
	OrganizationId uuid.UUID
	PartnerId      *string
	FirstName      string
	LastName       string
	DeletedAt      *time.Time

	// Currently only used to control display of the AI assist button in the UI - DO NOT use for anything else as it will be removed
	AiAssistEnabled bool
	Picture         string
}

func (u User) FullName() string {
	if u.FirstName == "" && u.LastName == "" {
		return ""
	}
	return u.FirstName + " " + u.LastName
}

type CreateUser struct {
	Email          string
	Role           Role
	OrganizationId uuid.UUID
	PartnerId      *string
	FirstName      string
	LastName       string
}

type UpdateUser struct {
	UserId    string
	Email     *string
	Role      *Role
	FirstName *string
	LastName  *string
}
