package models

import "github.com/google/uuid"

type IngestionOptions struct {
	ShouldMonitor          bool
	ShouldScreen           bool
	ContinuousScreeningIds []uuid.UUID
}

type IngestionResult struct {
	PreviousInternalId string
	NewInternalId      string
}

type IngestionResults map[string]IngestionResult
