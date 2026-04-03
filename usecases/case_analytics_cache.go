package usecases

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
)

const caseAnalyticsCacheTTL = time.Hour

// cachedTimeSeriesQuery handles the full cache flow for time-series analytics queries.
// It loads cached data from Redis, queries the DB only for missing date ranges,
// merges results, saves back to cache, and returns the filtered result.
// The inbox resolution is deferred until a cache miss requires a DB query.
func cachedTimeSeriesQuery[T analytics.Dated](
	ctx context.Context,
	uc CaseAnalyticsUsecase,
	filters dto.CaseAnalyticsFilters,
	queryName string,
	dbQuery func(ctx context.Context, exec repositories.Executor, f analytics.CaseAnalyticsFilter) ([]T, error),
) ([]T, error) {
	if !uc.license.Analytics {
		return []T{}, nil
	}

	exec := uc.executorFactory.NewExecutor()
	cache := exec.Cache(ctx)
	cacheKey, cacheEnabled := buildCaseAnalyticsCacheKey(ctx, cache, queryName, filters, false)

	// Try to load cached data
	var cached []T
	if cacheEnabled {
		if loaded, err := repositories.RedisLoadModel[[]T](ctx, cache, cacheKey); err == nil {
			cached = loaded
		}
	}

	// Determine what date ranges we need to fetch from DB
	requestedStart := filters.Start
	requestedEnd := filters.End

	_, tzOffset := filters.End.In(filters.Timezone).Zone()
	today := time.Now().Add(time.Duration(tzOffset) * time.Second).Truncate(24 * time.Hour)

	// Strip today from cached data — it must always be re-fetched
	cached = slices.DeleteFunc(cached, func(r T) bool {
		return !r.GetDate().Truncate(24 * time.Hour).Before(today)
	})

	var toFetch []dateRange
	if len(cached) == 0 {
		toFetch = []dateRange{{start: requestedStart, end: requestedEnd}}
	} else {
		cachedMin := cached[0].GetDate()
		cachedMax := cached[len(cached)-1].GetDate()

		if requestedStart.Before(cachedMin) {
			toFetch = append(toFetch, dateRange{start: requestedStart, end: cachedMin})
		}
		// Always fetch from the end of cached range (or today, whichever is earlier) to requestedEnd
		fetchFrom := cachedMax
		if today.Before(cachedMax) {
			fetchFrom = today
		}
		if requestedEnd.After(fetchFrom) {
			toFetch = append(toFetch, dateRange{start: fetchFrom, end: requestedEnd})
		}
	}

	// If nothing to fetch, return cached data filtered to range
	if len(toFetch) == 0 {
		return filterByDateRange(cached, requestedStart, requestedEnd), nil
	}

	// We need to query the DB — resolve inboxes now
	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return nil, err
	}
	if len(inboxIds) == 0 {
		return []T{}, nil
	}

	baseFilter := analytics.CaseAnalyticsFilter{
		OrgId:           filters.OrgId,
		InboxIds:        inboxIds,
		AssignedUserId:  filters.AssignedUserId,
		TzOffsetSeconds: tzOffset,
	}

	// Fetch missing ranges from DB
	var fresh []T
	for _, r := range toFetch {
		f := baseFilter
		f.Start = r.start
		f.End = r.end

		rows, err := dbQuery(ctx, exec, f)
		if err != nil {
			return nil, err
		}
		fresh = append(fresh, rows...)
	}

	// Merge cached + fresh, deduplicate by date (fresh wins)
	merged := mergeTimeSeries(cached, fresh)

	// Save merged result back to cache, excluding today (best-effort)
	if cacheEnabled {
		toCache := slices.DeleteFunc(slices.Clone(merged), func(r T) bool {
			return !r.GetDate().Truncate(24 * time.Hour).Before(today)
		})
		if len(toCache) > 0 {
			_ = cache.SaveModel(ctx, exec, cacheKey, toCache, caseAnalyticsCacheTTL)
		}
	}

	return filterByDateRange(merged, requestedStart, requestedEnd), nil
}

// cachedScalarQuery handles simple cache for non-time-series queries.
// On cache hit, the inbox resolution is skipped entirely.
func cachedScalarQuery[T any](
	ctx context.Context,
	uc CaseAnalyticsUsecase,
	filters dto.CaseAnalyticsFilters,
	queryName string,
	dbQuery func(ctx context.Context, exec repositories.Executor, f analytics.CaseAnalyticsFilter) (T, error),
) (T, error) {
	var zero T
	if !uc.license.Analytics {
		return zero, nil
	}

	exec := uc.executorFactory.NewExecutor()
	cache := exec.Cache(ctx)
	cacheKey, cacheEnabled := buildCaseAnalyticsCacheKey(ctx, cache, queryName, filters, true)

	// Try cache first
	if cacheEnabled {
		if cached, err := repositories.RedisLoadModel[T](ctx, cache, cacheKey); err == nil {
			return cached, nil
		}
	}

	// Cache miss — resolve inboxes and query DB
	inboxIds, err := uc.getFilteredInboxIds(ctx, exec, filters)
	if err != nil {
		return zero, err
	}
	if len(inboxIds) == 0 {
		return zero, nil
	}

	result, err := dbQuery(ctx, exec, analytics.CaseAnalyticsFilter{
		OrgId:          filters.OrgId,
		InboxIds:       inboxIds,
		AssignedUserId: filters.AssignedUserId,
		Start:          filters.Start,
		End:            filters.End,
	})
	if err != nil {
		return zero, err
	}

	if cacheEnabled {
		_ = cache.SaveModel(ctx, exec, cacheKey, result, caseAnalyticsCacheTTL)
	}

	return result, nil
}

// buildCaseAnalyticsCacheKey returns the cache key and whether caching is enabled.
// Returns ("", false) if no cache is available or no user is in context.
// For time-series queries, the key does not include start/end since the cache
// accumulates data across range expansions. For scalar queries, start/end are
// included since the result depends on the exact range.
func buildCaseAnalyticsCacheKey(
	ctx context.Context,
	cache *repositories.RedisExecutor,
	queryName string,
	filters dto.CaseAnalyticsFilters,
	includeRange bool,
) (string, bool) {
	if cache == nil {
		return "", false
	}

	creds, ok := utils.CredentialsFromCtx(ctx)
	if !ok {
		return "", false
	}

	inboxPart := "all"
	if filters.InboxId != nil {
		inboxPart = filters.InboxId.String()
	}

	assignedPart := "all"
	if filters.AssignedUserId != nil {
		assignedPart = *filters.AssignedUserId
	}

	parts := []string{
		"case-analytics",
		string(creds.ActorIdentity.UserId),
		filters.Timezone.String(),
		inboxPart,
		assignedPart,
		queryName,
	}

	if includeRange {
		parts = append(parts,
			filters.Start.Truncate(24*time.Hour).Format("2006-01-02"),
			filters.End.Truncate(24*time.Hour).Format("2006-01-02"),
		)
	}

	return cache.Key(parts...), true
}

type dateRange struct {
	start time.Time
	end   time.Time
}

// mergeTimeSeries combines cached and fresh data, deduplicating by date (fresh wins).
func mergeTimeSeries[T analytics.Dated](cached, fresh []T) []T {
	if len(fresh) == 0 {
		return cached
	}
	if len(cached) == 0 {
		return fresh
	}

	byDate := make(map[time.Time]T, len(cached)+len(fresh))
	for _, r := range cached {
		byDate[r.GetDate().Truncate(24*time.Hour)] = r
	}
	for _, r := range fresh {
		byDate[r.GetDate().Truncate(24*time.Hour)] = r
	}

	result := make([]T, 0, len(byDate))
	for _, r := range byDate {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].GetDate().Before(result[j].GetDate())
	})
	return result
}

// filterByDateRange returns only entries within [start, end).
func filterByDateRange[T analytics.Dated](data []T, start, end time.Time) []T {
	startDate := start.Truncate(24 * time.Hour)
	endDate := end.Truncate(24 * time.Hour)

	return slices.DeleteFunc(slices.Clone(data), func(r T) bool {
		d := r.GetDate().Truncate(24 * time.Hour)
		return d.Before(startDate) || !d.Before(endDate)
	})
}
