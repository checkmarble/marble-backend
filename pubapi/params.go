package pubapi

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func UuidParam(c *gin.Context, param string) (*uuid.UUID, error) {
	parsed, err := uuid.Parse(c.Param(param))
	if err != nil {
		return nil, errors.WithDetail(ErrInvalidPayload, err.Error())
	}

	return &parsed, nil
}

var dateFormats = []string{
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05-07:00",
}

type DateTime time.Time

func (b *DateTime) UnmarshalParam(param string) error {
	for _, df := range dateFormats {
		dt, err := time.Parse(df, param)
		if err != nil {
			continue
		}

		*b = DateTime(dt)

		return nil
	}

	return errors.WithDetailf(ErrInvalidPayload, "invalid datetime format, use yyyy-mm-ddThh:mm:ss+zz:zz")
}

func (b *DateTime) IsZero() bool {
	if b == nil {
		return true
	}
	return time.Time(*b).IsZero()
}
