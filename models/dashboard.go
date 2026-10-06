package models

import (
	"time"

	"github.com/google/uuid"
)

// Dashboard contains only aggregates and current names, never raw audit payloads.
type Dashboard struct {
	GeneratedAt time.Time
	Months      int
	RangeStart  time.Time
	Indicators  map[string]DashboardIndicator
}

type DashboardIndicator struct {
	Total            int
	Coverage         DashboardCoverage
	Weeks            []DashboardWeek
	Recent           []DashboardRecent
	RecentHasUnknown bool
}

// Earliest instants from which all lifecycle events (All) or all creation events
// (New) are proven retained. Nil means no proof at all.
type DashboardCoverage struct {
	AllSince *time.Time
	NewSince *time.Time
}

type DashboardWeek struct {
	Start   time.Time
	End     time.Time
	All     *int
	New     *int
	Partial bool
}

type DashboardRecent struct {
	Id        uuid.UUID
	Name      string
	CreatedAt time.Time
}
