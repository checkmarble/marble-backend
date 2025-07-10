package models

type HealthItemName string

const (
	DatabaseHealthItemName      HealthItemName = "database"
	OpenSanctionsHealthItemName HealthItemName = "open_sanctions"
	BigQueryHealthItemName      HealthItemName = "bigquery"
)

type HealthItemStatus struct {
	Name   HealthItemName
	Status bool
}

type HealthStatus struct {
	Statuses []HealthItemStatus
}

func (l HealthStatus) IsHealthy() bool {
	for _, status := range l.Statuses {
		if !status.Status {
			return false
		}
	}
	return true
}
