package models

type OpenSanctionCheckFilter map[string][]string

type OpenSanctionsQuery struct {
	Queries OpenSanctionCheckFilter `json:"queries"`
}

type SanctionCheckResult struct {
	Partial bool
	Count   int
	Matches []SanctionCheckResultMatch
}

type SanctionCheckResultMatch struct {
	Id       string
	Schema   string
	Datasets []string
	Names    []string
}
