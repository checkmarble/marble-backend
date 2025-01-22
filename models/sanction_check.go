package models

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsQuery struct {
	Queries   OpenSanctionCheckFilter         `json:"queries"`
	OrgConfig OrganizationOpenSanctionsConfig `json:"-"`
}

type SanctionCheck struct {
	Query OpenSanctionsQuery

	Id      string
	Partial bool
	Count   int
	Matches []SanctionCheckMatch
}

type SanctionCheckMatch struct {
	Payload []byte

	Id       string
	EntityId string
	QueryIds []string
	Datasets []string
}
