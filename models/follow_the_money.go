package models

import "github.com/cockroachdb/errors"

// ///////////////////////////////
// Follow The Money Entity
// ///////////////////////////////
type FollowTheMoneyEntity string

const (
	FollowTheMoneyEntityPerson       FollowTheMoneyEntity = "Person"
	FollowTheMoneyEntityCompany      FollowTheMoneyEntity = "Company"
	FollowTheMoneyEntityOrganization FollowTheMoneyEntity = "Organization"
	FollowTheMoneyEntityVessel       FollowTheMoneyEntity = "Vessel"
)

func FollowTheMoneyEntityFrom(s string) (FollowTheMoneyEntity, error) {
	switch s {
	case "Person":
		return FollowTheMoneyEntityPerson, nil
	case "Company":
		return FollowTheMoneyEntityCompany, nil
	case "Organization":
		return FollowTheMoneyEntityOrganization, nil
	case "Vessel":
		return FollowTheMoneyEntityVessel, nil
	default:
		return "", errors.Newf("unknown FTM entity: %s", s)
	}
}

func (e FollowTheMoneyEntity) String() string {
	return string(e)
}

// ///////////////////////////////
// Follow The Money Property
// ///////////////////////////////
type FollowTheMoneyProperty string

const (
	FollowTheMoneyPropertyName                 FollowTheMoneyProperty = "name"
	FollowTheMoneyPropertyEmail                FollowTheMoneyProperty = "email"
	FollowTheMoneyPropertyPhone                FollowTheMoneyProperty = "phone"
	FollowTheMoneyPropertyNationality          FollowTheMoneyProperty = "nationality"
	FollowTheMoneyPropertyBirthDate            FollowTheMoneyProperty = "birthDate"
	FollowTheMoneyPropertyBirthCountry         FollowTheMoneyProperty = "birthCountry"
	FollowTheMoneyPropertyDeathDate            FollowTheMoneyProperty = "deathDate"
	FollowTheMoneyPropertyCitizenship          FollowTheMoneyProperty = "citizenship"
	FollowTheMoneyPropertyPassportNumber       FollowTheMoneyProperty = "passportNumber"
	FollowTheMoneyPropertySocialSecurityNumber FollowTheMoneyProperty = "socialSecurityNumber"
	FollowTheMoneyPropertyAddress              FollowTheMoneyProperty = "address"
	FollowTheMoneyPropertyImoNumber            FollowTheMoneyProperty = "imoNumber"
	FollowTheMoneyPropertyRegistrationNumber   FollowTheMoneyProperty = "registrationNumber"
	FollowTheMoneyPropertyJurisdiction         FollowTheMoneyProperty = "jurisdiction"
	FollowTheMoneyPropertyIsinCode             FollowTheMoneyProperty = "isinCode"
	FollowTheMoneyPropertyWebsite              FollowTheMoneyProperty = "website"
	FollowTheMoneyPropertyCountry              FollowTheMoneyProperty = "country"
)

func FollowTheMoneyPropertyFrom(s string) (FollowTheMoneyProperty, error) {
	switch s {
	case "name":
		return FollowTheMoneyPropertyName, nil
	case "email":
		return FollowTheMoneyPropertyEmail, nil
	case "phone":
		return FollowTheMoneyPropertyPhone, nil
	case "nationality":
		return FollowTheMoneyPropertyNationality, nil
	case "birthDate":
		return FollowTheMoneyPropertyBirthDate, nil
	case "birthCountry":
		return FollowTheMoneyPropertyBirthCountry, nil
	case "deathDate":
		return FollowTheMoneyPropertyDeathDate, nil
	case "citizenship":
		return FollowTheMoneyPropertyCitizenship, nil
	case "passportNumber":
		return FollowTheMoneyPropertyPassportNumber, nil
	case "socialSecurityNumber":
		return FollowTheMoneyPropertySocialSecurityNumber, nil
	case "address":
		return FollowTheMoneyPropertyAddress, nil
	case "imoNumber":
		return FollowTheMoneyPropertyImoNumber, nil
	case "registrationNumber":
		return FollowTheMoneyPropertyRegistrationNumber, nil
	case "jurisdiction":
		return FollowTheMoneyPropertyJurisdiction, nil
	case "isinCode":
		return FollowTheMoneyPropertyIsinCode, nil
	case "website":
		return FollowTheMoneyPropertyWebsite, nil
	case "country":
		return FollowTheMoneyPropertyCountry, nil
	default:
		return "", errors.Newf("unknown FTM property: %s", s)
	}
}

func (p FollowTheMoneyProperty) String() string {
	return string(p)
}

var FollowTheMoneyEntityProperties = map[FollowTheMoneyEntity][]FollowTheMoneyProperty{
	FollowTheMoneyEntityPerson: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyEmail,
		FollowTheMoneyPropertyPhone,
		FollowTheMoneyPropertyNationality,
		FollowTheMoneyPropertyBirthDate,
		FollowTheMoneyPropertyBirthCountry,
		FollowTheMoneyPropertyDeathDate,
		FollowTheMoneyPropertyCitizenship,
		FollowTheMoneyPropertyPassportNumber,
		FollowTheMoneyPropertySocialSecurityNumber,
		FollowTheMoneyPropertyAddress,
		FollowTheMoneyPropertyCountry,
	},
	FollowTheMoneyEntityCompany: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyJurisdiction,
		FollowTheMoneyPropertyRegistrationNumber,
		FollowTheMoneyPropertyIsinCode,
		FollowTheMoneyPropertyEmail,
		FollowTheMoneyPropertyPhone,
		FollowTheMoneyPropertyWebsite,
		FollowTheMoneyPropertyAddress,
	},
	FollowTheMoneyEntityVessel: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyImoNumber,
		FollowTheMoneyPropertyRegistrationNumber,
		FollowTheMoneyPropertyAddress,
		FollowTheMoneyPropertyBirthCountry,
	},
}
