package models

type FeatureAvailability int

const (
	Enable FeatureAvailability = iota
	Disable
	Test
	UnknownFeatureAvailability
)

var ValidFeaturesAvailability = []FeatureAvailability{Enable, Disable, Test}

// Provide a string value for each outcome
func (f FeatureAvailability) String() string {
	switch f {
	case Enable:
		return "enable"
	case Disable:
		return "disable"
	case Test:
		return "test"
	}
	return "unknown"
}

// Provide an Outcome from a string value
func FeatureAvailabilityFrom(s string) FeatureAvailability {
	switch s {
	case "enable":
		return Enable
	case "disable":
		return Disable
	case "test":
		return Test
	}
	return UnknownFeatureAvailability
}
