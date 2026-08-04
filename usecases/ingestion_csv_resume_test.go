package usecases

import (
	"context"
	"encoding/csv"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/mocks"
	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testResumeBucket = "gs://test-ingestion"
	testResumeKey    = "uploads/org/transactions/upload"
)

// nopSeekCloser adapts a strings.Reader to the io.ReadSeekCloser that models.Blob carries.
type nopSeekCloser struct {
	*strings.Reader
}

func (nopSeekCloser) Close() error { return nil }

// makeHeaderReadUsecase returns a usecase whose blob repository serves `content` for the unranged
// GetBlob that readCsvHeader performs.
func makeHeaderReadUsecase(content string) (*IngestionUseCase, *mocks.MockBlobRepository) {
	blobRepository := new(mocks.MockBlobRepository)
	blobRepository.On("GetBlob", mock.Anything, testResumeBucket, testResumeKey, mock.Anything).
		Return(models.Blob{
			FileName:   testResumeKey,
			ReadCloser: nopSeekCloser{strings.NewReader(content)},
		}, nil)

	return &IngestionUseCase{
		blobRepository:     blobRepository,
		ingestionBucketUrl: testResumeBucket,
	}, blobRepository
}

func TestReadCsvHeader(t *testing.T) {
	const bom = "\ufeff"

	tests := []struct {
		name       string
		headerLine string
	}{
		{
			name:       "plain LF",
			headerLine: "object_id,updated_at,value\n",
		},
		{
			name:       "CRLF",
			headerLine: "object_id,updated_at,value\r\n",
		},
		{
			// Written by Excel and other Windows tools. The BOM must be trimmed off the first header
			// name, but must still be counted in the offset, otherwise a resume lands mid-field.
			name:       "UTF-8 BOM",
			headerLine: bom + "object_id,updated_at,value\n",
		},
		{
			name:       "quoted header name",
			headerLine: "object_id,\"updated_at\",value\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := tc.headerLine + "abc,2024-01-01T00:00:00Z,1\n"
			uc, blobRepository := makeHeaderReadUsecase(content)

			header, dataStart, err := uc.readCsvHeader(context.Background(), testResumeKey)
			require.NoError(t, err)

			assert.Equal(t, []string{"object_id", "updated_at", "value"}, header,
				"the BOM must not leak into the first header name")
			assert.Equal(t, int64(len(tc.headerLine)), dataStart,
				"dataStart must be the absolute offset of the first data row, BOM included")

			// The offset is only useful if it actually lands on the first data row.
			assert.Equal(t, "abc,2024-01-01T00:00:00Z,1\n", content[dataStart:])

			blobRepository.AssertExpectations(t)
		})
	}
}

func TestReadCsvHeaderEmptyFile(t *testing.T) {
	uc, _ := makeHeaderReadUsecase("")

	_, _, err := uc.readCsvHeader(context.Background(), testResumeKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error reading first row of CSV")
}

// TestCsvResumeFromCheckpointOffset covers the core invariant of resumable ingestion: the offset the
// loop persists, startOffset + csv.Reader.InputOffset(), is a record boundary, so reopening the file
// there yields every remaining row exactly once, with no gap and no duplicate.
func TestCsvResumeFromCheckpointOffset(t *testing.T) {
	tests := []struct {
		name string
		rows []string
	}{
		{
			name: "plain rows",
			rows: []string{
				"a,2024-01-01T00:00:00Z,1\n",
				"b,2024-01-02T00:00:00Z,2\n",
				"c,2024-01-03T00:00:00Z,3\n",
				"d,2024-01-04T00:00:00Z,4\n",
			},
		},
		{
			// A quoted field may contain the record separator. The checkpoint has to account for the
			// whole record, not stop at the first newline it sees.
			name: "quoted field containing a newline",
			rows: []string{
				"a,2024-01-01T00:00:00Z,1\n",
				"b,\"2024-01-02T00:00:00Z\",\"multi\nline\"\n",
				"c,2024-01-03T00:00:00Z,3\n",
				"d,2024-01-04T00:00:00Z,4\n",
			},
		},
		{
			name: "no trailing newline on the last row",
			rows: []string{
				"a,2024-01-01T00:00:00Z,1\n",
				"b,2024-01-02T00:00:00Z,2\n",
				"c,2024-01-03T00:00:00Z,3",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const headerLine = "\ufeffobject_id,updated_at,value\n"
			content := headerLine + strings.Join(tc.rows, "")

			uc, _ := makeHeaderReadUsecase(content)
			header, dataStart, err := uc.readCsvHeader(context.Background(), testResumeKey)
			require.NoError(t, err)

			// First pass: read two records, then checkpoint exactly the way ingestObjectsFromCSV does.
			startOffset := dataStart
			reader := newDataReader(content, startOffset, header)

			firstPass := readAll(t, reader, 2)
			checkpoint := startOffset + reader.InputOffset()

			// Second pass: resume from the checkpoint and drain the rest.
			resumed := newDataReader(content, checkpoint, header)
			secondPass := readAll(t, resumed, -1)

			// Reading the whole file in one pass is the reference: splitting it across a checkpoint
			// must produce exactly the same records in the same order.
			whole := readAll(t, newDataReader(content, dataStart, header), -1)

			assert.Equal(t, len(tc.rows), len(whole), "reference pass should read every row")
			assert.Equal(t, whole, append(firstPass, secondPass...),
				"resuming at the checkpoint must not skip or duplicate any row")
		})
	}
}

// TestCsvCheckpointAtEndOfFile documents why processUploadLog compares the checkpoint against the
// blob size: after the last record, the persisted offset equals the file length, and a range read
// starting there has nothing left to return.
func TestCsvCheckpointAtEndOfFile(t *testing.T) {
	const headerLine = "object_id,updated_at,value\n"
	content := headerLine + "a,2024-01-01T00:00:00Z,1\n"

	uc, _ := makeHeaderReadUsecase(content)
	header, dataStart, err := uc.readCsvHeader(context.Background(), testResumeKey)
	require.NoError(t, err)

	reader := newDataReader(content, dataStart, header)
	readAll(t, reader, -1)

	assert.Equal(t, int64(len(content)), dataStart+reader.InputOffset(),
		"the final checkpoint sits at EOF, which is why resuming there must be short-circuited")
}

func TestIngestionDeadline(t *testing.T) {
	t.Run("no deadline on the context", func(t *testing.T) {
		_, ok := ingestionDeadline(context.Background())
		assert.False(t, ok, "without a deadline the ingestion must run to completion rather than snooze")
	})

	t.Run("reserves the margin before river cancels", func(t *testing.T) {
		riverDeadline := time.Now().Add(time.Hour)
		ctx, cancel := context.WithDeadline(context.Background(), riverDeadline)
		defer cancel()

		deadline, ok := ingestionDeadline(ctx)
		require.True(t, ok)
		assert.Equal(t, riverDeadline.Add(-CSV_INGESTION_TIMEOUT_MARGIN), deadline)
		assert.True(t, deadline.Before(riverDeadline),
			"the ingestion must give up before river cancels the job, not after")
	})

	t.Run("already past the margin", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(),
			time.Now().Add(CSV_INGESTION_TIMEOUT_MARGIN/2))
		defer cancel()

		deadline, ok := ingestionDeadline(ctx)
		require.True(t, ok)
		assert.True(t, time.Now().After(deadline),
			"an attempt starting with less headroom than the margin should checkpoint after its first batch")
	})
}

// newDataReader mimics how ingestObjectsFromCSV consumes the file: a ranged read starting at an
// explicit offset, with no header row and no BOM handling.
func newDataReader(content string, offset int64, header []string) *csv.Reader {
	reader := csv.NewReader(strings.NewReader(content[offset:]))
	reader.FieldsPerRecord = len(header)
	return reader
}

// readAll reads up to `limit` records, or every remaining record when limit is negative.
func readAll(t *testing.T, reader *csv.Reader, limit int) [][]string {
	t.Helper()

	records := make([][]string, 0)
	for limit < 0 || len(records) < limit {
		record, err := reader.Read()
		if err == io.EOF { //nolint:errorlint
			break
		}
		require.NoError(t, err)
		records = append(records, record)
	}
	return records
}
