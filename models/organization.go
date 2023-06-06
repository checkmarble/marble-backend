package models

type Organization struct {
	ID           string
	Name         string
	DatabaseName string
}

type CreateOrganizationInput struct {
	Name         string
	DatabaseName string
}

type UpdateOrganizationInput struct {
	ID           string
	Name         *string
	DatabaseName *string
}
