package clock

import "time"

type Clock interface {
	Now() time.Time
}

type clock struct{}

func (c *clock) Now() time.Time {
	return time.Now()
}

func New() Clock {
	return &clock{}
}

type Mock struct {
	now time.Time
}

func NewMock(now time.Time) *Mock {
	return &Mock{
		now: now,
	}
}

func (m *Mock) Now() time.Time {
	return m.now
}
