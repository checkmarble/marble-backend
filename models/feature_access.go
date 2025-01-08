package models

type FeatureAccess int

const (
	Allow FeatureAccess = iota
	Disallow
	Test
	UnknownFeatureAccess
)

var ValidFeaturesAccess = []FeatureAccess{Allow, Disallow, Test}

// Provide a string value for each outcome
func (f FeatureAccess) String() string {
	switch f {
	case Allow:
		return "allow"
	case Disallow:
		return "disallow"
	case Test:
		return "test"
	}
	return "unknown"
}

// Provide an Outcome from a string value
func FeatureAccessFrom(s string) FeatureAccess {
	switch s {
	case "allow":
		return Allow
	case "disallow":
		return Disallow
	case "test":
		return Test
	}
	return UnknownFeatureAccess
}
