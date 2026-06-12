package repositories

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFileOffloadedReadWriter(t *testing.T) OffloadedReadWriter {
	t.Helper()

	return OffloadedReadWriter{
		Repository:          NewMarbleDbRepository(false, 0.3),
		BlobRepository:      NewBlobRepository(infra.GcpConfig{}),
		OffloadingBucketUrl: "file://" + t.TempDir(),
	}
}

func TestOffloadScreeningMatchPayloadRoundTrip(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)

	orgId := uuid.New()
	screeningId := uuid.NewString()
	matchId := uuid.NewString()
	payload := []byte(`{"score":0.9,"id":"entity-1"}`)

	require.NoError(t, rw.OffloadScreeningMatchPayload(ctx, orgId, screeningId, matchId, payload))

	got, err := rw.ReadOffloadedScreeningMatchPayload(ctx, orgId, screeningId, matchId)
	require.NoError(t, err)
	assert.JSONEq(t, string(payload), string(got))

	// Overwriting the same key (the enrichment case) replaces the payload.
	enriched := []byte(`{"score":0.9,"id":"entity-1","extra":"enriched"}`)
	require.NoError(t, rw.OffloadScreeningMatchPayload(ctx, orgId, screeningId, matchId, enriched))

	got, err = rw.ReadOffloadedScreeningMatchPayload(ctx, orgId, screeningId, matchId)
	require.NoError(t, err)
	assert.JSONEq(t, string(enriched), string(got))
}

func TestReadOffloadedScreeningMatchPayloadMissingKey(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)

	// A missing object is not an error: the caller falls back to the DB column.
	got, err := rw.ReadOffloadedScreeningMatchPayload(ctx, uuid.New(), uuid.NewString(), uuid.NewString())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestOffloadScreeningMatches(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)

	screening := models.ScreeningWithMatches{
		Screening: models.Screening{Id: uuid.NewString(), OrgId: uuid.New()},
		Matches: []models.ScreeningMatch{
			{EntityId: "entity-1", Payload: []byte(`{"score":0.9}`)},
			{EntityId: "entity-2", Payload: []byte(`{"score":0.4}`)},
		},
	}

	forInsert, err := rw.OffloadScreeningMatches(ctx, screening)
	require.NoError(t, err)
	require.Len(t, forInsert, 2)

	for i := range forInsert {
		// The matches handed to InsertScreening have an id and an empty payload (the read-time
		// signal that the payload was offloaded).
		assert.NotEmpty(t, forInsert[i].Id)
		assert.Empty(t, forInsert[i].Payload)

		// The same id is back-filled onto the caller's matches, which keep their payload for the
		// API response.
		assert.Equal(t, forInsert[i].Id, screening.Matches[i].Id)
		assert.NotEmpty(t, screening.Matches[i].Payload)

		// The payload was actually written to blob storage under the match's key.
		got, err := rw.ReadOffloadedScreeningMatchPayload(ctx, screening.OrgId, screening.Id, forInsert[i].Id)
		require.NoError(t, err)
		assert.JSONEq(t, string(screening.Matches[i].Payload), string(got))
	}
}

func TestHydrateScreeningMatches(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)

	orgId := uuid.New()
	screeningId := uuid.NewString()
	offloadedMatchId := uuid.NewString()
	legacyMatchId := uuid.NewString()
	missingMatchId := uuid.NewString()

	// An offloaded match: payload lives in blob storage, column is empty.
	payload := []byte(`{"score":0.7,"id":"entity-1"}`)
	require.NoError(t, rw.OffloadScreeningMatchPayload(ctx, orgId, screeningId, offloadedMatchId, payload))

	screenings := []models.ScreeningWithMatches{
		{
			Screening: models.Screening{Id: screeningId, OrgId: orgId},
			Matches: []models.ScreeningMatch{
				{Id: offloadedMatchId, ScreeningId: screeningId, Payload: nil},
				// A legacy match still has its payload in the column: left untouched, no blob read.
				{Id: legacyMatchId, ScreeningId: screeningId, Payload: []byte(`{"score":0.2}`)},
				// A match with no blob and no column payload: stays empty
				{Id: missingMatchId, ScreeningId: screeningId, Payload: nil},
			},
		},
	}

	require.NoError(t, rw.HydrateScreeningMatches(ctx, screenings))

	assert.JSONEq(t, string(payload), string(screenings[0].Matches[0].Payload))
	assert.JSONEq(t, `{"score":0.2}`, string(screenings[0].Matches[1].Payload))
	assert.Empty(t, screenings[0].Matches[2].Payload)
}

func TestHydrateScreeningMatchesDisabledIsNoOp(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)
	rw.OffloadingBucketUrl = ""

	screenings := []models.ScreeningWithMatches{
		{
			Screening: models.Screening{Id: uuid.NewString(), OrgId: uuid.New()},
			Matches:   []models.ScreeningMatch{{Id: uuid.NewString(), Payload: nil}},
		},
	}

	require.NoError(t, rw.HydrateScreeningMatches(ctx, screenings))
	assert.Empty(t, screenings[0].Matches[0].Payload)
}

func TestOffloadScreeningMatchesDisabledReturnsMatchesUnchanged(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)
	rw.OffloadingBucketUrl = ""

	screening := models.ScreeningWithMatches{
		Screening: models.Screening{Id: uuid.NewString(), OrgId: uuid.New()},
		Matches: []models.ScreeningMatch{
			{EntityId: "entity-1", Payload: []byte(`{"score":0.9}`)},
		},
	}

	forInsert, err := rw.OffloadScreeningMatches(ctx, screening)
	require.NoError(t, err)

	// When offloading is disabled the matches are returned unchanged, payload included, so the
	// repository writes them to the DB column as before.
	assert.Equal(t, screening.Matches, forInsert)
	assert.NotEmpty(t, forInsert[0].Payload)
}

func TestOffloadingDisabledIsNoOp(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)
	rw.OffloadingBucketUrl = ""

	assert.False(t, rw.IsOffloadingEnabled())

	// Writing is a no-op and reading returns nil so callers keep using the DB column.
	require.NoError(t, rw.OffloadScreeningMatchPayload(ctx, uuid.New(), "sc", "m", []byte(`{"score":1}`)))

	got, err := rw.ReadOffloadedScreeningMatchPayload(ctx, uuid.New(), "sc", "m")
	require.NoError(t, err)
	assert.Nil(t, got)
}
