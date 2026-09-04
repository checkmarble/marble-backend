package usecases

import (
	"context"
	"encoding/csv"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestIsRetryableIngestionError(t *testing.T) {
	assert.False(t, isRetryableIngestionError(nil, true))
	assert.False(t, isRetryableIngestionError(models.BadParameterError, false))
	assert.False(t, isRetryableIngestionError(&pgconn.PgError{Code: "23505"}, false))
	assert.False(t, isRetryableIngestionError(&csv.ParseError{}, false))
	assert.True(t, isRetryableIngestionError(context.DeadlineExceeded, false))
	assert.True(t, isRetryableIngestionError(errors.Wrap(context.Canceled, "batch interrupted"), false))
	assert.True(t, isRetryableIngestionError(errors.New("unknown storage error"), false))
}
