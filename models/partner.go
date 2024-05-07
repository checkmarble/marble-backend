package models

import "time"

type Partner struct {
	Id        string
	CreatedAt time.Time
	Name      string
}

type PartnerCreateInput struct {
	Name string
}

type PartnerUpdateInput struct {
	Name string
}
