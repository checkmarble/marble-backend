package repositories

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models/analytics"
)

func TestBuildCaseSlaStatusByDateQuery_ExcludesNullSLA(t *testing.T) {
	// Arrange: create a filter with test values
	orgId := uuid.New()
	inboxId := uuid.New()
	now := time.Now()

	filters := analytics.CaseAnalyticsFilter{
		OrgId:           orgId,
		InboxIds:        []uuid.UUID{inboxId},
		TzOffsetSeconds: 0,
		Start:           now.Add(-24 * time.Hour),
		End:             now,
	}

	// Act: build the query
	query, err := buildCaseSlaStatusByDateQuery(filters)
	require.NoError(t, err)

	// Convert to SQL to inspect the query structure
	sql, args, err := query.ToSql()
	require.NoError(t, err)

	// Assert: verify the query includes the critical WHERE clause for non-null SLA
	require.Contains(t, sql, "i.sla IS NOT NULL", "Query must filter for inboxes with SLA configured (i.sla IS NOT NULL)")

	// Assert: verify old "or i.sla is null" clauses are removed
	require.NotContains(t, sql, "i.sla is null or", "Old 'i.sla is null or' clauses should be removed")
	require.NotContains(t, sql, "or i.sla is null", "Old 'or i.sla is null' clauses should be removed")

	// Assert: verify org_id and inbox_id filters are still present
	require.Contains(t, sql, "c.org_id", "Query must filter by organization ID")
	require.Contains(t, sql, "c.inbox_id", "Query must filter by inbox IDs")

	// Assert: verify date range filters are still present
	require.Contains(t, sql, "c.created_at", "Query must filter by creation date range")

	// Assert: verify the expected aggregate functions are present
	require.Contains(t, sql, "completed_within_sla", "Query must return completed_within_sla column")
	require.Contains(t, sql, "sla_breached", "Query must return sla_breached column")
	require.Contains(t, sql, "still_open_within_sla", "Query must return still_open_within_sla column")

	// Verify args contain expected values (at minimum 2 UUIDs for org and inbox, 2 timestamps, and event type/status)
	require.GreaterOrEqual(t, len(args), 5, "Query args must include at least org ID, inbox ID, date range, and event type")
}

func TestBuildCaseSlaStatusByDateQuery_WithAssignedUserId(t *testing.T) {
	// Arrange: create a filter with an assigned user ID
	orgId := uuid.New()
	inboxId := uuid.New()
	assignedUserId := "test-user-123"
	now := time.Now()

	filters := analytics.CaseAnalyticsFilter{
		OrgId:          orgId,
		InboxIds:       []uuid.UUID{inboxId},
		AssignedUserId: &assignedUserId,
		Start:          now.Add(-24 * time.Hour),
		End:            now,
	}

	// Act: build the query
	query, err := buildCaseSlaStatusByDateQuery(filters)
	require.NoError(t, err)

	// Convert to SQL to inspect the query structure
	sql, args, err := query.ToSql()
	require.NoError(t, err)

	// Assert: verify assigned_to filter is present
	require.Contains(t, sql, "c.assigned_to", "Query must filter by assigned user when provided")

	// Verify we have more args than the base case (should include the assigned user ID)
	require.GreaterOrEqual(t, len(args), 6, "Query args must include assigned user ID in addition to base filters")
}
