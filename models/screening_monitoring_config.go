package models

import (
	"time"

	"github.com/google/uuid"
)

// Configuration for screening monitoring for an organization.
// Defines a set of datasets that are used for the monitoring.
type ScreeningMonitoringConfig struct {
	Id          uuid.UUID
	OrgId       string
	Name        string
	Description *string

	// Dataset that are used for the monitoring
	Datasets []string

	// Threshold used in matching score, between 0 and 100
	MatchThreshold int

	MatchLimit int
	Enabled    bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateScreeningMonitoringConfig struct {
	OrgId          string
	Name           string
	Description    *string
	Datasets       []string
	MatchThreshold int
	MatchLimit     int
}

type UpdateScreeningMonitoringConfig struct {
	Name           *string
	Description    *string
	Datasets       *[]string
	MatchThreshold *int
	MatchLimit     *int
	Enabled        *bool
}
