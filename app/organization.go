package app

type Organization struct {
	ID   string
	Name string
}

type CreateOrganisation struct {
	Name         string
	DatabaseName string
}
