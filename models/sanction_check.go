package models

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsQuery struct {
	Queries   OpenSanctionCheckFilter         `json:"queries"`
	OrgConfig OrganizationOpenSanctionsConfig `json:"-"`
}

type SanctionCheckExecution struct {
	Query OpenSanctionsQuery

	Id      string
	Partial bool
	Count   int
	Matches []SanctionCheckExecutionMatch
}

type SanctionCheckExecutionMatch struct {
	Raw []byte

	Id       string
	Schema   string
	EntityId string
	QueryIds []string
	Datasets []string
}
