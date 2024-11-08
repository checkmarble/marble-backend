package models

type CreatePhantomDecisionInput struct {
	OrganizationId     string
	Scenario           Scenario
	ClientObject       ClientObject
	Pivot              *Pivot
	TriggerObjectTable string
}
