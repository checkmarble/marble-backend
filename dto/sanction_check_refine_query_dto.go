package dto

import (
	"github.com/checkmarble/marble-backend/models"
)

type RefineQueryDto struct {
	Thing        *RefineQueryBase         `json:"thing,omitempty" binding:"excluded_with_all=Person Organization Vehicle"`
	Person       *RefineQueryPerson       `json:"person,omitempty" binding:"excluded_with_all=Thing Organization Vehicle"`
	Organization *RefineQueryOrganization `json:"organization,omitempty" binding:"excluded_with_all=Person Thing Vehicle"`
	Vehicle      *RefineQueryVehicle      `json:"vehicle,omitempty" binding:"excluded_with_all=Thing Person Organization"`
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

	BirthDate   string `json:"birth_date"`
	Nationality string `json:"nationality"`
	IdNumber    string `json:"id_number"`
	Address     string `json:"address"`
}

type RefineQueryOrganization struct {
	RefineQueryBase

	Country            string `json:"country"`
	RegistrationNumber string `json:"registration_number"`
	Address            string `json:"address"`
}

type RefineQueryVehicle struct {
	RefineQueryBase

	RegistrationNumber string `json:"registration_number"`
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
