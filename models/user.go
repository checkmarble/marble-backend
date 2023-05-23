package models

type User struct {
	UserId         string
	Email          string
	FirebaseUid    string
	Role           Role
	OrganizationId string
}
