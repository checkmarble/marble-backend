package dto

type CreateInitialOrg struct {
	Organization string `json:"organization" binding:"required,min=1"`
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"omitempty,min=8"`
	Firstname    string `json:"firstname" binding:"required,min=1"`
	Lastname     string `json:"lastname" binding:"required,min=1"`
}
