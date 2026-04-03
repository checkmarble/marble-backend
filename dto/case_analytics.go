package dto

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type CaseAnalyticsFilters struct {
	OrgId uuid.UUID `json:"-"`

	TimezoneName string `json:"timezone"` //nolint:tagliatelle
	Timezone     *time.Location

	Start          time.Time  `json:"start" binding:"required"`
	End            time.Time  `json:"end" binding:"required"`
	InboxId        *uuid.UUID `json:"inbox_id"`
	AssignedUserId *string    `json:"assigned_user_id"`
}

func (f *CaseAnalyticsFilters) Validate() error {
	tz, err := time.LoadLocation(f.TimezoneName)
	if err != nil {
		return errors.Newf("invalid timezone name %s", f.TimezoneName)
	}
	f.Timezone = tz

	if f.End.Before(f.Start) {
		return errors.New("end must be after start")
	}

	return nil
}
