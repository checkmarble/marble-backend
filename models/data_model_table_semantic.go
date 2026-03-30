package models

type SemanticType string

const (
	SemanticTypeUnset       SemanticType = ""
	SemanticTypeUnknown     SemanticType = "unknown"
	SemanticTypePerson      SemanticType = "person"
	SemanticTypeCompany     SemanticType = "company"
	SemanticTypeAccount     SemanticType = "account"
	SemanticTypeTransaction SemanticType = "transaction"
	SemanticTypeEvent       SemanticType = "event"
	SemanticTypePartner     SemanticType = "partner"
	SemanticTypeOther       SemanticType = "other"
)

var validSemanticTypes = map[SemanticType]bool{
	SemanticTypePerson:      true,
	SemanticTypeCompany:     true,
	SemanticTypeAccount:     true,
	SemanticTypeTransaction: true,
	SemanticTypeEvent:       true,
	SemanticTypePartner:     true,
	SemanticTypeOther:       true,
}

// Input validation. The unset value is valid only for old table. New table after this change must have a semantic type defined
func (s SemanticType) IsValid() bool {
	return validSemanticTypes[s]
}

func SemanticTypeFrom(s string) SemanticType {
	if s == "" {
		return SemanticTypeUnset
	}
	st := SemanticType(s)
	if validSemanticTypes[st] {
		return st
	}
	return SemanticTypeUnknown
}

// Need for validation, some semantic type require a BelongsTo link to a Party
func (s SemanticType) IsParty() bool {
	return s == SemanticTypePerson || s == SemanticTypeCompany
}

func (s SemanticType) RequiresBelongsToLink() bool {
	return s == SemanticTypeTransaction || s == SemanticTypeEvent || s == SemanticTypeAccount
}
