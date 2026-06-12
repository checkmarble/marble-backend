package repositories

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
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

// /////////////////////////////
// Offload screening payload //
// /////////////////////////////
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

// ///////////////////////////////////////
// Offload continuous screening payload //
// ///////////////////////////////////////

func TestOffloadContinuousScreeningEntity(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)

	orgId := uuid.New()
	csId := uuid.New()
	entityPayload := []byte(`{"id":"entity-1","caption":"ACME"}`)

	// Offloaded: returns nil (to store an empty column) and writes the payload to blob storage.
	stored, err := rw.OffloadContinuousScreeningEntity(ctx, orgId, csId, entityPayload)
	require.NoError(t, err)
	assert.Empty(t, stored)

	got, err := rw.ReadOffloadedContinuousScreeningEntityPayload(ctx, orgId, csId)
	require.NoError(t, err)
	assert.JSONEq(t, string(entityPayload), string(got))

	// Disabled: returns the payload unchanged so it is written to the DB column.
	rw.OffloadingBucketUrl = ""
	stored, err = rw.OffloadContinuousScreeningEntity(ctx, orgId, csId, entityPayload)
	require.NoError(t, err)
	assert.JSONEq(t, string(entityPayload), string(stored))
}

func TestOffloadContinuousScreeningMatches(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)

	orgId := uuid.New()
	csId := uuid.New()
	matches := []models.ScreeningMatch{
		{EntityId: "match-1", Payload: []byte(`{"score":0.9}`)},
		{EntityId: "match-2", Payload: []byte(`{"score":0.4}`)},
	}

	offloaded, err := rw.OffloadContinuousScreeningMatches(ctx, orgId, csId, matches)
	require.NoError(t, err)
	require.Len(t, offloaded, 2)

	for i, m := range offloaded {
		// Each offloaded match has a pre-assigned id and an empty payload.
		assert.NotEmpty(t, m.Id)
		assert.Empty(t, m.Payload)

		matchId, err := uuid.Parse(m.Id)
		require.NoError(t, err)

		got, err := rw.ReadOffloadedContinuousScreeningMatchPayload(ctx, orgId, csId, matchId)
		require.NoError(t, err)
		assert.JSONEq(t, string(matches[i].Payload), string(got))
	}
}

func TestOffloadContinuousScreeningMatchesDisabledIsNoOp(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)
	rw.OffloadingBucketUrl = ""

	matches := []models.ScreeningMatch{{EntityId: "match-1", Payload: []byte(`{"score":0.9}`)}}

	offloaded, err := rw.OffloadContinuousScreeningMatches(ctx, uuid.New(), uuid.New(), matches)
	require.NoError(t, err)

	// Unchanged: payloads stay so the repository writes them to the DB column as before.
	assert.Equal(t, matches, offloaded)
	assert.NotEmpty(t, offloaded[0].Payload)
}

func TestHydrateContinuousScreeningMatches(t *testing.T) {
	ctx := context.Background()
	rw := newFileOffloadedReadWriter(t)

	orgId := uuid.New()
	csId := uuid.New()
	offloadedMatchId := uuid.New()
	legacyMatchId := uuid.New()

	entityPayload := []byte(`{"id":"entity-1"}`)
	matchPayload := []byte(`{"score":0.7}`)
	require.NoError(t, rw.OffloadContinuousScreeningEntityPayload(ctx, orgId, csId, entityPayload))
	require.NoError(t, rw.OffloadContinuousScreeningMatchPayload(ctx, orgId, csId, offloadedMatchId, matchPayload))

	screenings := []models.ContinuousScreeningWithMatches{
		{
			ContinuousScreening: models.ContinuousScreening{
				Id:                   csId,
				OrgId:                orgId,
				OpenSanctionEntityId: utils.Ptr("entity-1"),
			},
			Matches: []models.ContinuousScreeningMatch{
				{Id: offloadedMatchId, ContinuousScreeningId: csId, Payload: nil},
				{Id: legacyMatchId, ContinuousScreeningId: csId, Payload: []byte(`{"score":0.2}`)},
			},
		},
	}

	require.NoError(t, rw.HydrateContinuousScreeningEntity(ctx, &screenings[0]))
	require.NoError(t, rw.HydrateContinuousScreeningMatch(ctx, &screenings[0]))

	assert.JSONEq(t, string(entityPayload), string(screenings[0].OpenSanctionEntityPayload))
	assert.JSONEq(t, string(matchPayload), string(screenings[0].Matches[0].Payload))
	assert.JSONEq(t, `{"score":0.2}`, string(screenings[0].Matches[1].Payload))
}
