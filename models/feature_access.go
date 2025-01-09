package models

type FeatureAccess int

const (
	Allowed FeatureAccess = iota
	Premium
	Test
	UnknownFeatureAccess
)

var ValidFeaturesAccess = []FeatureAccess{Allowed, Premium, Test}

// Provide a string value for each outcome
func (f FeatureAccess) String() string {
	switch f {
	case Allowed:
		return "allowed"
	case Premium:
		return "premium"
	case Test:
		return "test"
	}
	return "unknown"
}

// Provide an Outcome from a string value
func FeatureAccessFrom(s string) FeatureAccess {
	switch s {
	case "allowed":
		return Allowed
	case "premium":
		return Premium
	case "test":
		return Test
	}
	return UnknownFeatureAccess
}
