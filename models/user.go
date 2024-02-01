package models

import "time"

type UserId string

type User struct {
	UserId         UserId
	Email          string
	Role           Role
	OrganizationId string
	FirstName      string
	LastName       string
	DeletedAt      *time.Time
}

type CreateUser struct {
	Email          string
	Role           Role
	OrganizationId string
	FirstName      string
	LastName       string
}

type UpdateUser struct {
	UserId    UserId
	Email     string
	Role      Role
	FirstName string
	LastName  string
}
