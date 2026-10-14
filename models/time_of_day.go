package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// TimeOfDayRange is a daily time range, (un)marshaled as two times of day in
// HHMM form ([900, 1750] is from 9:00am to 5:50pm). Both ends are inclusive,
// to the minute, like the other caveats. A start later than the end spans
// midnight ([2200, 600] is from 10:00pm to 6:00am).
type TimeOfDayRange [2]int

func NewTimeOfDayRange(start, end int) (TimeOfDayRange, error) {
	timeOfDay := TimeOfDayRange{start, end}

	if err := timeOfDay.Validate(); err != nil {
		return TimeOfDayRange{}, err
	}

	return timeOfDay, nil
}

func (r TimeOfDayRange) Start() int {
	return r[0]
}

func (r TimeOfDayRange) End() int {
	return r[1]
}

func (r TimeOfDayRange) Validate() error {
	for _, hhmm := range r {
		if hhmm < 0 || hhmm/100 > 23 || hhmm%100 > 59 {
			return fmt.Errorf("invalid time of day %d, expected HHMM: %w", hhmm, BadParameterError)
		}
	}

	return nil
}

func (r TimeOfDayRange) Contains(t time.Time) bool {
	hhmm := t.Hour()*100 + t.Minute()

	if r.Start() <= r.End() {
		return hhmm >= r.Start() && hhmm <= r.End()
	}

	return hhmm >= r.Start() || hhmm <= r.End()
}

func (r *TimeOfDayRange) UnmarshalJSON(b []byte) error {
	var times []int

	if err := json.Unmarshal(b, &times); err != nil {
		return err
	}

	if len(times) != 2 {
		return fmt.Errorf("time of day range must have a start and an end, got %d values: %w",
			len(times), BadParameterError)
	}

	timeOfDay, err := NewTimeOfDayRange(times[0], times[1])
	if err != nil {
		return err
	}

	*r = timeOfDay

	return nil
}
