package tracking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/require"
)

type settingsRepositoryStub struct {
	t        *testing.T
	metadata *models.Metadata
	err      error
	reads    int
	writes   []models.Metadata
	writeErr error
}

func (r *settingsRepositoryStub) GetMetadata(_ context.Context, _ repositories.Executor, orgID *uuid.UUID, key models.MetadataKey) (*models.Metadata, error) {
	r.reads++
	require.Nil(r.t, orgID)
	require.Equal(r.t, models.MetadataKeyUsageTrackingEnabled, key)
	return r.metadata, r.err
}

func (r *settingsRepositoryStub) UpsertMetadata(_ context.Context, _ repositories.Executor, metadata models.Metadata) error {
	r.writes = append(r.writes, metadata)
	if r.writeErr == nil {
		r.metadata = &metadata
	}
	return r.writeErr
}

func TestSettingsEnabled(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata *models.Metadata
		enabled  bool
	}{
		{name: "absent defaults to enabled", enabled: true},
		{name: "explicit opt in", metadata: &models.Metadata{Value: "true"}, enabled: true},
		{name: "explicit opt out", metadata: &models.Metadata{Value: "false"}, enabled: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &settingsRepositoryStub{t: t, metadata: tc.metadata}
			factory := executor_factory.NewExecutorFactoryStub()
			t.Cleanup(factory.Mock.Close)
			settings := NewSettings(repo, factory, false)

			require.Equal(t, tc.enabled, settings.Enabled(context.Background()))
			require.Equal(t, tc.enabled, settings.Enabled(context.Background()))
			require.Equal(t, 1, repo.reads, "also cache absent and false values")
		})
	}
}

func TestSettingsMarbleSaasIgnoresOptOut(t *testing.T) {
	repo := &settingsRepositoryStub{t: t, metadata: &models.Metadata{Value: "false"}}
	settings := NewSettings(repo, nil, true)

	require.True(t, settings.Enabled(context.Background()))
	require.Zero(t, repo.reads)
}

func TestSettingsRetriesAfterErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata *models.Metadata
		err      error
	}{
		{name: "database error", err: errors.New("database unavailable")},
		{name: "invalid value", metadata: &models.Metadata{Value: "invalid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &settingsRepositoryStub{t: t, metadata: tc.metadata, err: tc.err}
			factory := executor_factory.NewExecutorFactoryStub()
			t.Cleanup(factory.Mock.Close)
			settings := NewSettings(repo, factory, false)

			require.False(t, settings.Enabled(context.Background()))
			repo.metadata = &models.Metadata{Value: "true"}
			repo.err = nil
			require.True(t, settings.Enabled(context.Background()))
			require.Equal(t, 2, repo.reads)
		})
	}
}

func TestSettingsRefreshesAfterCacheExpiry(t *testing.T) {
	repo := &settingsRepositoryStub{t: t}
	factory := executor_factory.NewExecutorFactoryStub()
	t.Cleanup(factory.Mock.Close)
	settings := NewSettings(repo, factory, false)
	settings.cache = expirable.NewLRU[models.MetadataKey, bool](1, nil, 20*time.Millisecond)

	require.True(t, settings.Enabled(context.Background()))
	repo.metadata = &models.Metadata{Value: "false"}
	require.True(t, settings.Enabled(context.Background()))
	require.Equal(t, 1, repo.reads)

	require.Eventually(t, func() bool {
		return !settings.Enabled(context.Background())
	}, time.Second, time.Millisecond)
	require.Equal(t, 2, repo.reads)
}

func TestSettingsSetEnabledUpdatesOnlyUsageTrackingAndCache(t *testing.T) {
	repo := &settingsRepositoryStub{t: t}
	factory := executor_factory.NewExecutorFactoryStub()
	t.Cleanup(factory.Mock.Close)
	settings := NewSettings(repo, factory, false)
	ctx := context.Background()
	credentials := models.Credentials{Role: models.ADMIN}
	require.True(t, settings.Enabled(ctx))

	for _, enabled := range []bool{false, true} {
		require.NoError(t, settings.SetEnabled(ctx, credentials, enabled))
		require.Equal(t, enabled, settings.Enabled(ctx))
	}

	require.Equal(t, []models.Metadata{
		{Key: models.MetadataKeyUsageTrackingEnabled, Value: "false"},
		{Key: models.MetadataKeyUsageTrackingEnabled, Value: "true"},
	}, repo.writes)
	require.Equal(t, 1, repo.reads, "updates take effect without waiting for expiry or another DB read")
}

func TestSettingsSetEnabledRejectsUnauthorizedAndSaasChanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		role models.Role
		saas bool
	}{
		{name: "viewer", role: models.VIEWER},
		{name: "builder", role: models.BUILDER},
		{name: "publisher", role: models.PUBLISHER},
		{name: "API client", role: models.API_CLIENT},
		{name: "no role", role: models.NO_ROLE},
		{name: "SaaS admin", role: models.ADMIN, saas: true},
		{name: "SaaS Marble admin", role: models.MARBLE_ADMIN, saas: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &settingsRepositoryStub{t: t}
			settings := NewSettings(repo, nil, tc.saas)
			require.ErrorIs(t, settings.SetEnabled(context.Background(), models.Credentials{Role: tc.role}, false), models.ForbiddenError)
			require.Empty(t, repo.writes)
		})
	}
}

func TestSettingsSetEnabledKeepsCacheOnWriteFailure(t *testing.T) {
	repo := &settingsRepositoryStub{t: t, writeErr: errors.New("write failed")}
	factory := executor_factory.NewExecutorFactoryStub()
	t.Cleanup(factory.Mock.Close)
	settings := NewSettings(repo, factory, false)
	ctx := context.Background()
	require.True(t, settings.Enabled(ctx))

	require.ErrorIs(t, settings.SetEnabled(ctx, models.Credentials{Role: models.ADMIN}, false), repo.writeErr)
	require.True(t, settings.Enabled(ctx))
	require.Equal(t, 1, repo.reads)
}
