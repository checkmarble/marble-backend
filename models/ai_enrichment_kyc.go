package models

import "time"

type Citation struct {
	Title  string
	Domain string
	Url    string
	Date   time.Time
}

type AiEnrichmentKYC struct {
	Analysis   string
	EntityName string
	Citations  []Citation
}
