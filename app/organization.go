package app

type Organization struct {
	ID   string
	Name string
}

type CreateOrganizationInput struct {
	Name         string
	DatabaseName string
}
