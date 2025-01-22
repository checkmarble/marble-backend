package models

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsQuery struct {
	Queries   OpenSanctionCheckFilter         `json:"queries"`
	OrgConfig OrganizationOpenSanctionsConfig `json:"-"`
}

type SanctionCheck struct {
	Id          string
	DecisionId  string
	Query       OpenSanctionsQuery
	Partial     bool
	Count       int
	Status      string
	IsManual    bool
	RequestedBy *string
	Matches     []SanctionCheckMatch
}

type SanctionCheckMatch struct {
	Id              string
	SanctionCheckId string
	EntityId        string
	Status          string
	QueryIds        []string
	Datasets        []string
	Payload         []byte
}

type SanctionCheckMatchUpdate struct {
	Status string
}
