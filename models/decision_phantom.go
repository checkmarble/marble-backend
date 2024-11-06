package models

type CreatePhantomDecisionInput struct {
	OrganizationId     string
	Scenario           Scenario
	ClientObject       *ClientObject
	Payload            ClientObject
	Pivot              *Pivot
	TriggerObjectTable string
}
