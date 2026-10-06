package dbmodels

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

// The dashboard query aggregates in PostgreSQL and returns one JSON document.
type DbDashboard struct {
	GeneratedAt time.Time                       `json:"generated_at"`
	Months      int                             `json:"months"`
	RangeStart  time.Time                       `json:"range_start"`
	Indicators  map[string]DbDashboardIndicator `json:"indicators"`
}

type DbDashboardIndicator struct {
	Total            int                 `json:"total"`
	AllSince         *time.Time          `json:"all_since"`
	NewSince         *time.Time          `json:"new_since"`
	Weeks            []DbDashboardWeek   `json:"weeks"`
	Recent           []DbDashboardRecent `json:"recent"`
	RecentHasUnknown bool                `json:"recent_has_unknown"`
}

type DbDashboardWeek struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	All     *int      `json:"all"`
	New     *int      `json:"new"`
	Partial bool      `json:"partial"`
}

type DbDashboardRecent struct {
	Id        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Dates are normalized to UTC even when the database session uses another zone.
func AdaptDashboard(db DbDashboard) models.Dashboard {
	indicators := make(map[string]models.DashboardIndicator, len(db.Indicators))
	for entity, indicator := range db.Indicators {
		weeks := make([]models.DashboardWeek, len(indicator.Weeks))
		for i, week := range indicator.Weeks {
			weeks[i] = models.DashboardWeek{
				Start:   week.Start.UTC(),
				End:     week.End.UTC(),
				All:     week.All,
				New:     week.New,
				Partial: week.Partial,
			}
		}
		recent := make([]models.DashboardRecent, len(indicator.Recent))
		for i, record := range indicator.Recent {
			recent[i] = models.DashboardRecent{
				Id:        record.Id,
				Name:      record.Name,
				CreatedAt: record.CreatedAt.UTC(),
			}
		}
		indicators[entity] = models.DashboardIndicator{
			Total: indicator.Total,
			Coverage: models.DashboardCoverage{
				AllSince: utcPtr(indicator.AllSince),
				NewSince: utcPtr(indicator.NewSince),
			},
			Weeks:            weeks,
			Recent:           recent,
			RecentHasUnknown: indicator.RecentHasUnknown,
		}
	}

	return models.Dashboard{
		GeneratedAt: db.GeneratedAt.UTC(),
		Months:      db.Months,
		RangeStart:  db.RangeStart.UTC(),
		Indicators:  indicators,
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	return utils.Ptr(t.UTC())
}
