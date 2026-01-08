package models

type IngestionResult struct {
	PreviousInternalId string
	NewInternalId      string
}

type IngestionResults map[string]IngestionResult
