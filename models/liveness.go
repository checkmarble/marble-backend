package models

type LivenessItemName string

const (
	DatabaseLivenessItemName      LivenessItemName = "Database"
	OpenSanctionsLivenessItemName LivenessItemName = "Open Sanctions"
)

type LivenessItemStatus struct {
	Name   LivenessItemName
	IsLive bool
	Error  error
}

type LivenessStatus struct {
	Statuses []LivenessItemStatus
}

func (l LivenessStatus) IsLive() bool {
	for _, status := range l.Statuses {
		if !status.IsLive {
			return false
		}
	}
	return true
}
