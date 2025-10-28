package models

// ///////////////////////////////
// Follow The Money Entity
// ///////////////////////////////
type FollowTheMoneyEntity string

const (
	FollowTheMoneyEntityUnknown      FollowTheMoneyEntity = "Unknown"
	FollowTheMoneyEntityPerson       FollowTheMoneyEntity = "Person"
	FollowTheMoneyEntityCompany      FollowTheMoneyEntity = "Company"
	FollowTheMoneyEntityOrganization FollowTheMoneyEntity = "Organization"
	FollowTheMoneyEntityVessel       FollowTheMoneyEntity = "Vessel"
)

func FollowTheMoneyEntityFrom(s string) FollowTheMoneyEntity {
	switch s {
	case "Person":
		return FollowTheMoneyEntityPerson
	case "Company":
		return FollowTheMoneyEntityCompany
	case "Organization":
		return FollowTheMoneyEntityOrganization
	case "Vessel":
		return FollowTheMoneyEntityVessel
	default:
		return FollowTheMoneyEntityUnknown
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
	FollowTheMoneyPropertyUnknown              FollowTheMoneyProperty = "unknown"
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
	FollowTheMoneyPropertyCountry              FollowTheMoneyProperty = "country"
	FollowTheMoneyPropertyCageCode             FollowTheMoneyProperty = "cageCode"
	FollowTheMoneyPropertyPermId               FollowTheMoneyProperty = "permId"
	FollowTheMoneyPropertyImoNumber            FollowTheMoneyProperty = "imoNumber"
	FollowTheMoneyPropertyGiiNumber            FollowTheMoneyProperty = "giiNumber"
	FollowTheMoneyPropertyVATCode              FollowTheMoneyProperty = "vatCode"
	FollowTheMoneyPropertyRegistrationNumber   FollowTheMoneyProperty = "registrationNumber"
	FollowTheMoneyPropertyJurisdiction         FollowTheMoneyProperty = "jurisdiction"
	FollowTheMoneyPropertyIsinCode             FollowTheMoneyProperty = "isinCode"
	FollowTheMoneyPropertyWebsite              FollowTheMoneyProperty = "website"
)

func FollowTheMoneyPropertyFrom(s string) FollowTheMoneyProperty {
	switch s {
	case "name":
		return FollowTheMoneyPropertyName
	case "email":
		return FollowTheMoneyPropertyEmail
	case "phone":
		return FollowTheMoneyPropertyPhone
	case "nationality":
		return FollowTheMoneyPropertyNationality
	case "birthDate":
		return FollowTheMoneyPropertyBirthDate
	case "birthCountry":
		return FollowTheMoneyPropertyBirthCountry
	case "deathDate":
		return FollowTheMoneyPropertyDeathDate
	case "citizenship":
		return FollowTheMoneyPropertyCitizenship
	case "passportNumber":
		return FollowTheMoneyPropertyPassportNumber
	case "socialSecurityNumber":
		return FollowTheMoneyPropertySocialSecurityNumber
	case "address":
		return FollowTheMoneyPropertyAddress
	case "country":
		return FollowTheMoneyPropertyCountry
	case "cageCode":
		return FollowTheMoneyPropertyCageCode
	case "permId":
		return FollowTheMoneyPropertyPermId
	case "imoNumber":
		return FollowTheMoneyPropertyImoNumber
	case "giiNumber":
		return FollowTheMoneyPropertyGiiNumber
	case "vatCode":
		return FollowTheMoneyPropertyVATCode
	case "registrationNumber":
		return FollowTheMoneyPropertyRegistrationNumber
	case "jurisdiction":
		return FollowTheMoneyPropertyJurisdiction
	case "isinCode":
		return FollowTheMoneyPropertyIsinCode
	case "website":
		return FollowTheMoneyPropertyWebsite
	default:
		return FollowTheMoneyPropertyUnknown
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
	FollowTheMoneyEntityOrganization: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyBirthCountry,
		FollowTheMoneyPropertyEmail,
		FollowTheMoneyPropertyPhone,
		FollowTheMoneyPropertyWebsite,
		FollowTheMoneyPropertyRegistrationNumber,
		FollowTheMoneyPropertyAddress,
	},
	// I am not sure about this one, do we need to search Vessel? do we need to add more properties?
	FollowTheMoneyEntityVessel: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyImoNumber,
		FollowTheMoneyPropertyRegistrationNumber,
		FollowTheMoneyPropertyAddress,
		FollowTheMoneyPropertyBirthCountry,
	},
}
