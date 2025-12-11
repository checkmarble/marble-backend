package dto

import "github.com/google/uuid"

type OrgImport struct {
	Org         ImportOrg          `json:"org"`
	Admins      []CreateUser       `json:"admins"`
	DataModel   ImportDataModel    `json:"data_model"`
	Scenarios   []ImportScenario   `json:"scenarios"`
	Tags        []ImportTag        `json:"tags"`
	CustomLists []ImportCustomList `json:"custom_lists"`
	Inboxes     []InboxDto         `json:"inboxes"`
	Workflows   []ImportWorkflow   `json:"workflows"`
}

type ImportOrg struct {
	Name string `json:"name"`
	UpdateOrganizationBodyDto
}

type ImportDataModel struct {
	Tables []Table        `json:"tables"`
	Links  []LinkToSingle `json:"links"`
	Pivots []Pivot        `json:"pivots"`
}

type ImportTag struct {
	CreateTagBody

	Id string `json:"id"`
}

type ImportCustomList struct {
	CustomList

	Values []string `json:"values"`
}

type ImportScenario struct {
	Scenario  ScenarioDto              `json:"scenario"`
	Iteration ScenarioIterationBodyDto `json:"iteration"`
}

type ImportWorkflow struct {
	WorkflowRuleDto

	ScenarioId uuid.UUID              `json:"scenario_id"`
	Conditions []WorkflowConditionDto `json:"conditions"`
	Actions    []WorkflowActionDto    `json:"actions"`
}
