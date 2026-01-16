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
	FollowTheMoneyEntityAirplane     FollowTheMoneyEntity = "Airplane"
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
	case "Airplane":
		return FollowTheMoneyEntityAirplane
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
	FollowTheMoneyPropertyUnknown              FollowTheMoneyProperty = "Unknown"
	FollowTheMoneyPropertyName                 FollowTheMoneyProperty = "name"
	FollowTheMoneyPropertyFirstName            FollowTheMoneyProperty = "firstName"
	FollowTheMoneyPropertyLastName             FollowTheMoneyProperty = "lastName"
	FollowTheMoneyPropertyEmail                FollowTheMoneyProperty = "email"
	FollowTheMoneyPropertyPhone                FollowTheMoneyProperty = "phone"
	FollowTheMoneyPropertyNationality          FollowTheMoneyProperty = "nationality"
	FollowTheMoneyPropertyBirthDate            FollowTheMoneyProperty = "birthDate"
	FollowTheMoneyPropertyBirthCountry         FollowTheMoneyProperty = "birthCountry"
	FollowTheMoneyPropertyCitizenship          FollowTheMoneyProperty = "citizenship"
	FollowTheMoneyPropertyPassportNumber       FollowTheMoneyProperty = "passportNumber"
	FollowTheMoneyPropertySocialSecurityNumber FollowTheMoneyProperty = "socialSecurityNumber"
	FollowTheMoneyPropertyIdNumber             FollowTheMoneyProperty = "idNumber"
	FollowTheMoneyPropertyImoNumber            FollowTheMoneyProperty = "imoNumber"
	FollowTheMoneyPropertyRegistrationNumber   FollowTheMoneyProperty = "registrationNumber"
	FollowTheMoneyPropertyJurisdiction         FollowTheMoneyProperty = "jurisdiction"
	FollowTheMoneyPropertyIsinCode             FollowTheMoneyProperty = "isinCode"
	FollowTheMoneyPropertyWebsite              FollowTheMoneyProperty = "website"
	FollowTheMoneyPropertyCountry              FollowTheMoneyProperty = "country"
	FollowTheMoneyPropertyMainCountry          FollowTheMoneyProperty = "mainCountry"
	FollowTheMoneyPropertyFlag                 FollowTheMoneyProperty = "flag"
	FollowTheMoneyPropertyNotes                FollowTheMoneyProperty = "notes"
)

func FollowTheMoneyPropertyFrom(s string) FollowTheMoneyProperty {
	switch s {
	case "name":
		return FollowTheMoneyPropertyName
	case "firstName":
		return FollowTheMoneyPropertyFirstName
	case "lastName":
		return FollowTheMoneyPropertyLastName
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
	case "citizenship":
		return FollowTheMoneyPropertyCitizenship
	case "passportNumber":
		return FollowTheMoneyPropertyPassportNumber
	case "socialSecurityNumber":
		return FollowTheMoneyPropertySocialSecurityNumber
	case "imoNumber":
		return FollowTheMoneyPropertyImoNumber
	case "registrationNumber":
		return FollowTheMoneyPropertyRegistrationNumber
	case "jurisdiction":
		return FollowTheMoneyPropertyJurisdiction
	case "isinCode":
		return FollowTheMoneyPropertyIsinCode
	case "website":
		return FollowTheMoneyPropertyWebsite
	case "country":
		return FollowTheMoneyPropertyCountry
	case "mainCountry":
		return FollowTheMoneyPropertyMainCountry
	case "idNumber":
		return FollowTheMoneyPropertyIdNumber
	case "notes":
		return FollowTheMoneyPropertyNotes
	case "flag":
		return FollowTheMoneyPropertyFlag
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
		FollowTheMoneyPropertyFirstName,
		FollowTheMoneyPropertyLastName,
		FollowTheMoneyPropertyEmail,
		FollowTheMoneyPropertyPhone,
		FollowTheMoneyPropertyNationality,
		FollowTheMoneyPropertyBirthDate,
		FollowTheMoneyPropertyBirthCountry,
		FollowTheMoneyPropertyCitizenship,
		FollowTheMoneyPropertyPassportNumber,
		FollowTheMoneyPropertySocialSecurityNumber,
		FollowTheMoneyPropertyIdNumber,
		FollowTheMoneyPropertyCountry,
	},
	FollowTheMoneyEntityCompany: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyRegistrationNumber,
		FollowTheMoneyPropertyJurisdiction,
		FollowTheMoneyPropertyCountry,
		FollowTheMoneyPropertyIsinCode,
		FollowTheMoneyPropertyEmail,
		FollowTheMoneyPropertyPhone,
		FollowTheMoneyPropertyWebsite,
		FollowTheMoneyPropertyMainCountry,
	},
	FollowTheMoneyEntityOrganization: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyRegistrationNumber,
		FollowTheMoneyPropertyJurisdiction,
		FollowTheMoneyPropertyCountry,
		FollowTheMoneyPropertyEmail,
		FollowTheMoneyPropertyPhone,
		FollowTheMoneyPropertyWebsite,
		FollowTheMoneyPropertyMainCountry,
	},
	FollowTheMoneyEntityVessel: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyImoNumber,
		FollowTheMoneyPropertyRegistrationNumber,
		FollowTheMoneyPropertyCountry,
		FollowTheMoneyPropertyFlag,
	},
	FollowTheMoneyEntityAirplane: {
		FollowTheMoneyPropertyName,
		FollowTheMoneyPropertyRegistrationNumber,
		FollowTheMoneyPropertyCountry,
	},
}
