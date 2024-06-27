package models

import (
	"time"

	"github.com/guregu/null/v5"
)

type Partner struct {
	Id        string
	CreatedAt time.Time
	Name      string
	Bic       string
}

type PartnerCreateInput struct {
	Name string
	Bic  string
}

type PartnerUpdateInput struct {
	Name null.String
	Bic  null.String
}

type PartnerFilters struct {
	Bic null.String
}
