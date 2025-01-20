package models

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsQuery struct {
	Queries OpenSanctionCheckFilter `json:"queries"`
}

type SanctionCheckExecution struct {
	Partial bool
	Count   int
	Matches []SanctionCheckExecutionMatch
}

type SanctionCheckExecutionMatch struct {
	Id       string
	Schema   string
	Datasets []string
	Names    []string
}
