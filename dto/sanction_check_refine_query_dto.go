package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type RefineQueryDto struct {
	Thing        *RefineQueryBase         `json:"Thing,omitempty" binding:"required_without_all=Person Organization Vehicle,excluded_with=Person Organization Vehicle"` //nolint:tagliatelle
	Person       *RefineQueryPerson       `json:"Person,omitempty" binding:"required_without_all=Thing Organization Vehicle,excluded_with=Thing Organization Vehicle"`  //nolint:tagliatelle
	Organization *RefineQueryOrganization `json:"Organization,omitempty" binding:"required_without_all=Thing Person Vehicle,excluded_with=Person Thing Vehicle"`        //nolint:tagliatelle
	Vehicle      *RefineQueryVehicle      `json:"Vehicle,omitempty" binding:"required_without_all=Thing Person Organization,excluded_with=Thing Person Organization"`   //nolint:tagliatelle
}

func (dto RefineQueryDto) Type() string {
	switch {
	case dto.Person != nil:
		return "Person"
	case dto.Organization != nil:
		return "Organization"
	case dto.Vehicle != nil:
		return "Vehicle"
	}

	return "Thing"
}

type RefineQueryBase struct {
	Name string `json:"name"`
}

type RefineQueryPerson struct {
	RefineQueryBase

	BirthDate   string `json:"birthDate"` //nolint:tagliatelle
	Nationality string `json:"nationality"`
	IdNumber    string `json:"idNumber"` //nolint:tagliatelle
	Address     string `json:"address"`
}

type RefineQueryOrganization struct {
	RefineQueryBase

	Country            string `json:"country"`
	RegistrationNumber string `json:"registrationNumber"` //nolint:tagliatelle
	Address            string `json:"address"`
}

type RefineQueryVehicle struct {
	RefineQueryBase

	RegistrationNumber string `json:"registrationNumber"` //nolint:tagliatelle
}

func AdaptRefineQueryDto(dto RefineQueryDto) models.OpenSanctionCheckFilter {
	filter := models.OpenSanctionCheckFilter{}

	assign := func(queryField, value string) {
		if value != "" {
			filter[queryField] = []string{value}
		}
	}

	switch {
	case dto.Thing != nil:
		assign("name", dto.Thing.Name)
	case dto.Person != nil:
		assign("name", dto.Person.Name)
		assign("birthDate", dto.Person.BirthDate)
		assign("nationality", dto.Person.Nationality)
		assign("idNumber", dto.Person.IdNumber)
		assign("address", dto.Person.Address)
	case dto.Organization != nil:
		assign("name", dto.Organization.Name)
		assign("country", dto.Organization.Country)
		assign("registrationNumber", dto.Organization.RegistrationNumber)
		assign("address", dto.Organization.Address)
	case dto.Vehicle != nil:
		assign("name", dto.Vehicle.Name)
		assign("registrationNumber", dto.Vehicle.RegistrationNumber)
	}

	return filter
}
