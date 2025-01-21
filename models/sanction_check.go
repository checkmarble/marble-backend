package models

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsQuery struct {
	Queries      OpenSanctionCheckFilter         `json:"-"`
	QueryPayload []byte                          `json:"queries"` //nolint:tagliatelle
	OrgConfig    OrganizationOpenSanctionsConfig `json:"-"`
}

type SanctionCheckExecution struct {
	Query OpenSanctionsQuery

	Id      string
	Partial bool
	Count   int
	Matches []SanctionCheckExecutionMatch
}

type SanctionCheckExecutionMatch struct {
	Payload []byte

	Id       string
	EntityId string
	QueryIds []string
	Datasets []string
}
