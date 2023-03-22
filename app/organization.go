package app

type Organization struct {
	ID   string
	Name string

	Tokens    map[string]string //map[tokenID]token
	DataModel DataModel
	Scenarios map[string]Scenario //map[scenarioID]Scenario
}
