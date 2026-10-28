package repositories

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutInBatches_SplitsRowsInOrder(t *testing.T) {
	rows := make([]int, 1210)
	for i := range rows {
		rows[i] = i
	}

	var batches [][]int
	err := putInBatches(context.Background(), rows, METRICS_INSERT_BATCH_SIZE, func(_ context.Context, batch []int) error {
		batches = append(batches, batch)
		return nil
	})
	require.NoError(t, err)

	require.Len(t, batches, 3)
	assert.Len(t, batches[0], 500)
	assert.Len(t, batches[1], 500)
	assert.Len(t, batches[2], 210)

	var inserted []int
	for _, batch := range batches {
		inserted = append(inserted, batch...)
	}
	assert.Equal(t, rows, inserted)
}

func TestPutInBatches_NoRowsInsertsNothing(t *testing.T) {
	calls := 0
	err := putInBatches(context.Background(), []int{}, METRICS_INSERT_BATCH_SIZE, func(_ context.Context, _ []int) error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 0, calls)
}

func TestPutInBatches_StopsAtFirstFailingBatch(t *testing.T) {
	putErr := errors.New("request too large")
	calls := 0
	err := putInBatches(context.Background(), make([]int, 1500), METRICS_INSERT_BATCH_SIZE, func(_ context.Context, _ []int) error {
		calls++
		if calls == 2 {
			return putErr
		}
		return nil
	})
	require.ErrorIs(t, err, putErr)
	assert.Equal(t, 2, calls)
}
