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
	code, message := ingestionFailureDetails(inputErr, nil)
	assert.Equal(t, models.IngestionFailureInvalidInput, code)
	assert.Equal(t, "invalid amount at line 42", message)

	internalErr := errors.New("relation private_schema.internal_table does not exist")
	code, message = ingestionFailureDetails(nil, internalErr)
	assert.Equal(t, models.IngestionFailureInternalError, code)
	assert.Equal(t, "ingestion failed due to an internal error", message)

	code, message = ingestionFailureDetails(nil, errors.Wrap(models.NotFoundError, "private resource name"))
	assert.Equal(t, models.IngestionFailureInvalidInput, code)
	assert.Equal(t, "a resource required for this ingestion was not found", message)
}

func TestIngestionFailureMessage(t *testing.T) {
	tests := map[models.IngestionFailureCode]string{
		models.IngestionFailureGlobalTimeout:     "ingestion did not complete before its deadline",
		models.IngestionFailureUploadNotReceived: "upload was not received before its deadline",
		models.IngestionFailureFileTooLarge:      "uploaded file exceeds the 10 GB limit",
		models.IngestionFailureInvalidInput:      "the ingestion input is invalid",
		models.IngestionFailureInternalError:     "ingestion failed due to an internal error",
	}
	for code, expected := range tests {
		t.Run(string(code), func(t *testing.T) {
			assert.Equal(t, expected, ingestionFailureMessage(code))
		})
	}
}
