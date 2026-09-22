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
	assert.False(t, isRetryableIngestionError(nil))
	assert.False(t, isRetryableIngestionError(models.BadParameterError))
	assert.False(t, isRetryableIngestionError(&pgconn.PgError{Code: "23505"}))
	assert.False(t, isRetryableIngestionError(&csv.ParseError{}))
	assert.True(t, isRetryableIngestionError(context.DeadlineExceeded))
	assert.True(t, isRetryableIngestionError(errors.Wrap(context.Canceled, "batch interrupted")))
	assert.True(t, isRetryableIngestionError(errors.New("unknown storage error")))
}

func TestIngestionFailureDetails(t *testing.T) {
	inputErr := errors.WithDetail(models.BadParameterError, "invalid amount at line 42")
	assert.Equal(t, models.IngestionFailureInvalidInput, ingestionFailureCode(inputErr, nil))

	internalErr := errors.New("relation private_schema.internal_table does not exist")
	assert.Equal(t, models.IngestionFailureInternalError, ingestionFailureCode(nil, internalErr))
}
