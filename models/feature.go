package models

import "time"

type Feature struct {
	Id        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type CreateFeatureInput struct {
	Id   string
	Name string
}

type CreateFeatureAttributes struct {
	Name string
}

type UpdateFeatureInput struct {
	Id   string
	Name string
}

type UpdateFeatureAttributes struct {
	Id   string
	Name string
}
