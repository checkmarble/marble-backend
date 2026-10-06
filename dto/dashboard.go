package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
)

type Dashboard struct {
	GeneratedAt time.Time                     `json:"generated_at"`
	Months      int                           `json:"months"`
	RangeStart  time.Time                     `json:"range_start"`
	Indicators  map[string]DashboardIndicator `json:"indicators"`
}

type DashboardIndicator struct {
	Total            int               `json:"total"`
	Coverage         DashboardCoverage `json:"coverage"`
	Weeks            []DashboardWeek   `json:"weeks"`
	Recent           []DashboardRecent `json:"recent"`
	RecentHasUnknown bool              `json:"recent_has_unknown"`
}

type DashboardCoverage struct {
	AllSince *time.Time `json:"all_since"`
	NewSince *time.Time `json:"new_since"`
}

type DashboardWeek struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	All     *int      `json:"all"`
	New     *int      `json:"new"`
	Partial bool      `json:"partial"`
}

type DashboardRecent struct {
	Id        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func AdaptDashboard(m models.Dashboard) Dashboard {
	indicators := make(map[string]DashboardIndicator, len(m.Indicators))
	for entity, indicator := range m.Indicators {
		weeks := make([]DashboardWeek, len(indicator.Weeks))
		for i, week := range indicator.Weeks {
			weeks[i] = DashboardWeek{
				Start:   week.Start,
				End:     week.End,
				All:     week.All,
				New:     week.New,
				Partial: week.Partial,
			}
		}
		recent := make([]DashboardRecent, len(indicator.Recent))
		for i, record := range indicator.Recent {
			recent[i] = DashboardRecent{
				Id:        record.Id,
				Name:      record.Name,
				CreatedAt: record.CreatedAt,
			}
		}
		indicators[entity] = DashboardIndicator{
			Total: indicator.Total,
			Coverage: DashboardCoverage{
				AllSince: indicator.Coverage.AllSince,
				NewSince: indicator.Coverage.NewSince,
			},
			Weeks:            weeks,
			Recent:           recent,
			RecentHasUnknown: indicator.RecentHasUnknown,
		}
	}

	return Dashboard{
		GeneratedAt: m.GeneratedAt,
		Months:      m.Months,
		RangeStart:  m.RangeStart,
		Indicators:  indicators,
	}
}
