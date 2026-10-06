package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gavv/httpexpect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var dashboardEntities = []string{"organizations", "tenants", "users"}

func dashboardAuth(t *testing.T) *httpexpect.Expect {
	t.Helper()
	e := httpexpect.Default(t, testServer.URL)
	token := e.POST("/token").WithHeader("Authorization", "Bearer "+firebaseDummyToken(marbleAdminEmail)).Expect().Status(http.StatusOK).JSON().Object().Value("access_token").String().Raw()
	return e.Builder(func(req *httpexpect.Request) { req.WithHeader("Authorization", "Bearer "+token) })
}

func dashboardRead(t *testing.T, auth *httpexpect.Expect, months int) dto.Dashboard {
	t.Helper()
	payload := auth.GET("/admin/dashboard").WithQuery("months", months).Expect().Status(http.StatusOK).Body().Raw()
	var result dto.Dashboard
	require.NoError(t, json.Unmarshal([]byte(payload), &result))
	return result
}

func dashboardExec(t *testing.T, query string, args ...any) {
	t.Helper()
	_, err := pgPool.Exec(context.Background(), query, args...)
	require.NoError(t, err)
}

// Explicitly controlled test history; this never runs outside the disposable DB.
// Coverage triggers are disabled so rewriting event times does not move coverage.
func dashboardHistoryFixture(t *testing.T, fn func(func(string, ...any))) {
	t.Helper()
	ctx := context.Background()
	tx, err := pgPool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := tx.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	exec("ALTER TABLE audit.audit_events DISABLE TRIGGER dashboard_coverage_on_update")
	exec("ALTER TABLE audit.audit_events DISABLE TRIGGER dashboard_coverage_on_delete")
	fn(exec)
	exec("ALTER TABLE audit.audit_events ENABLE TRIGGER dashboard_coverage_on_update")
	exec("ALTER TABLE audit.audit_events ENABLE TRIGGER dashboard_coverage_on_delete")
	require.NoError(t, tx.Commit(ctx))
}

func dashboardSetCoverage(t *testing.T, since time.Time) {
	t.Helper()
	dashboardExec(t, "UPDATE audit.dashboard_coverage SET since=$1", since)
}

type dashboardCoverageRow struct {
	Entity, Measure string
	Since           time.Time
}

func dashboardCoverageRows(t *testing.T) map[string]time.Time {
	t.Helper()
	rows, err := pgPool.Query(context.Background(), "SELECT entity, measure, since FROM audit.dashboard_coverage")
	require.NoError(t, err)
	defer rows.Close()
	result := map[string]time.Time{}
	for rows.Next() {
		var row dashboardCoverageRow
		require.NoError(t, rows.Scan(&row.Entity, &row.Measure, &row.Since))
		result[row.Entity+"/"+row.Measure] = row.Since
	}
	require.NoError(t, rows.Err())
	return result
}

func dashboardMonday(now time.Time) time.Time {
	now = now.UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
}

func dashboardCurrent(t *testing.T, d dto.Dashboard, entity string) dto.DashboardWeek {
	t.Helper()
	weeks := d.Indicators[entity].Weeks
	require.NotEmpty(t, weeks)
	return weeks[len(weeks)-1]
}

func dashboardWeek(t *testing.T, d dto.Dashboard, entity string, start time.Time) dto.DashboardWeek {
	t.Helper()
	for _, week := range d.Indicators[entity].Weeks {
		if week.Start.Equal(start) {
			return week
		}
	}
	require.FailNow(t, "week not found", "%s %s", entity, start)
	return dto.DashboardWeek{}
}

func TestDashboardLifecycleAndAuthorization(t *testing.T) {
	auth := dashboardAuth(t)
	httpexpect.Default(t, testServer.URL).GET("/admin/dashboard").Expect().Status(http.StatusUnauthorized)
	for _, invalid := range []string{"0", "2", "24", "invalid", ""} {
		auth.GET("/admin/dashboard").WithQuery("months", invalid).Expect().Status(http.StatusBadRequest)
	}
	auth.GET("/admin/dashboard").Expect().Status(http.StatusOK).JSON().Object().Value("months").Number().IsEqual(6)
	dashboardSetCoverage(t, dashboardMonday(time.Now()).AddDate(0, 0, -7))
	before := dashboardRead(t, auth, 6)
	tenant, org, user := uuid.New(), uuid.New(), uuid.New()
	dashboardExec(t, "INSERT INTO tenants(id,name) VALUES($1,$2)", tenant, tenant.String())
	dashboardExec(t, "INSERT INTO organizations(id,name,tenant_id) VALUES($1,$2,$3)", org, org.String(), tenant)
	dashboardExec(t, "INSERT INTO users(id,email,role) VALUES($1,$2,$3)", user, user.String()+"@dashboard.test", models.VIEWER)
	after := dashboardRead(t, auth, 6)
	for _, entity := range dashboardEntities {
		require.Equal(t, before.Indicators[entity].Total+1, after.Indicators[entity].Total)
		require.Equal(t, *dashboardCurrent(t, before, entity).New+1, *dashboardCurrent(t, after, entity).New)
		require.Equal(t, after.Indicators[entity].Total, *dashboardCurrent(t, after, entity).All)
	}
	var userAudits int
	require.NoError(t, pgPool.QueryRow(context.Background(), `SELECT count(*) FROM audit.audit_events WHERE "table"='users' AND entity_id=$1 AND operation='INSERT'`, user).Scan(&userAudits))
	require.Equal(t, 1, userAudits, "one event from actorless user creation")
	// A soft-deleted org is still in the organization management list.
	dashboardExec(t, "UPDATE organizations SET deleted_at=now() WHERE id=$1", org)
	require.Equal(t, after.Indicators["organizations"].Total, dashboardRead(t, auth, 6).Indicators["organizations"].Total)
	// Repeated soft deletion is not a second removal; restoration is not creation.
	for i := 0; i < 2; i++ {
		dashboardExec(t, "UPDATE users SET deleted_at=now() WHERE id=$1", user)
		dashboardExec(t, "UPDATE tenants SET deleted_at=now() WHERE id=$1", tenant)
	}
	deleted := dashboardRead(t, auth, 6)
	for _, entity := range []string{"tenants", "users"} {
		require.Equal(t, before.Indicators[entity].Total, deleted.Indicators[entity].Total)
		require.Equal(t, before.Indicators[entity].Total, *dashboardCurrent(t, deleted, entity).All)
		require.Equal(t, *dashboardCurrent(t, after, entity).New, *dashboardCurrent(t, deleted, entity).New)
	}
	dashboardExec(t, "UPDATE users SET deleted_at=NULL WHERE id=$1", user)
	dashboardExec(t, "UPDATE tenants SET deleted_at=NULL WHERE id=$1", tenant)
	restored := dashboardRead(t, auth, 6)
	for _, entity := range []string{"tenants", "users"} {
		require.Equal(t, after.Indicators[entity].Total, restored.Indicators[entity].Total)
		require.Equal(t, *dashboardCurrent(t, after, entity).New, *dashboardCurrent(t, restored, entity).New)
	}
	// Physical deletion of an active record decreases All, never New.
	dashboardExec(t, "DELETE FROM organizations WHERE id=$1", org)
	dashboardExec(t, "DELETE FROM users WHERE id=$1", user)
	dashboardExec(t, "DELETE FROM tenants WHERE id=$1", tenant)
	final := dashboardRead(t, auth, 6)
	for _, entity := range dashboardEntities {
		require.Equal(t, before.Indicators[entity].Total, final.Indicators[entity].Total)
		require.Equal(t, before.Indicators[entity].Total, *dashboardCurrent(t, final, entity).All)
		require.Equal(t, *dashboardCurrent(t, after, entity).New, *dashboardCurrent(t, final, entity).New)
	}
	// A regular account cannot fetch global aggregates.
	e := httpexpect.Default(t, testServer.URL)
	viewer := uuid.New()
	dashboardExec(t, "INSERT INTO users(id,email,role) VALUES($1,$2,$3)", viewer, viewer.String()+"@dashboard.test", models.VIEWER)
	dashboardExec(t, "INSERT INTO grants(id,principal_type,principal_id,principal_authority,role) VALUES($1,'user',$2,'marble','VIEWER')", uuid.New(), viewer.String())
	token := e.POST("/token").WithHeader("Authorization", "Bearer "+firebaseDummyToken(viewer.String()+"@dashboard.test")).Expect().Status(http.StatusOK).JSON().Object().Value("access_token").String().Raw()
	e.GET("/admin/dashboard").WithHeader("Authorization", "Bearer "+token).Expect().Status(http.StatusForbidden)
	dashboardExec(t, "DELETE FROM users WHERE id=$1", viewer)
}

func TestDashboardRollbackHasNoObservableEffect(t *testing.T) {
	auth := dashboardAuth(t)
	ctx := context.Background()
	dashboardSetCoverage(t, dashboardMonday(time.Now()).AddDate(0, 0, -7))
	existing := uuid.New()
	dashboardExec(t, "INSERT INTO tenants(id,name) VALUES($1,$2)", existing, existing.String())
	before := dashboardRead(t, auth, 6)

	tx, err := pgPool.Begin(ctx)
	require.NoError(t, err)
	tenant, org, user := uuid.New(), uuid.New(), uuid.New()
	_, err = tx.Exec(ctx, "INSERT INTO tenants(id,name) VALUES($1,$2)", tenant, tenant.String())
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO organizations(id,name,tenant_id) VALUES($1,$2,$3)", org, org.String(), tenant)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO users(id,email,role) VALUES($1,$2,$3)", user, user.String()+"@dashboard.test", models.VIEWER)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "DELETE FROM tenants WHERE id=$1", existing)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))

	after := dashboardRead(t, auth, 6)
	for _, entity := range dashboardEntities {
		require.Equal(t, before.Indicators[entity].Total, after.Indicators[entity].Total)
		require.Equal(t, *dashboardCurrent(t, before, entity).New, *dashboardCurrent(t, after, entity).New)
		require.Equal(t, *dashboardCurrent(t, before, entity).All, *dashboardCurrent(t, after, entity).All)
		require.Equal(t, before.Indicators[entity].Recent, after.Indicators[entity].Recent)
	}
	var rolledBackAudits int
	require.NoError(t, pgPool.QueryRow(ctx, "SELECT count(*) FROM audit.audit_events WHERE entity_id = ANY($1)", []uuid.UUID{tenant, org, user}).Scan(&rolledBackAudits))
	require.Zero(t, rolledBackAudits)
	dashboardExec(t, "DELETE FROM tenants WHERE id=$1", existing)
}

func TestDashboardEditsDoNotChangeCounts(t *testing.T) {
	auth := dashboardAuth(t)
	dashboardSetCoverage(t, dashboardMonday(time.Now()).AddDate(0, 0, -7))
	tenant, org, user := uuid.New(), uuid.New(), uuid.New()
	dashboardExec(t, "INSERT INTO tenants(id,name) VALUES($1,$2)", tenant, tenant.String())
	dashboardExec(t, "INSERT INTO organizations(id,name,tenant_id) VALUES($1,$2,$3)", org, org.String(), tenant)
	dashboardExec(t, "INSERT INTO users(id,email,organization_id,role) VALUES($1,$2,$3,$4)", user, user.String()+"@dashboard.test", org, models.ADMIN)
	before := dashboardRead(t, auth, 6)

	dashboardExec(t, "UPDATE tenants SET name=$2 WHERE id=$1", tenant, "renamed-"+tenant.String())
	dashboardExec(t, "UPDATE organizations SET name=$2 WHERE id=$1", org, "renamed-"+org.String())
	dashboardExec(t, "UPDATE users SET first_name='renamed' WHERE id=$1", user)
	dashboardExec(t, "UPDATE organizations SET default_scenario_timezone='Europe/Paris' WHERE id=$1", org)
	grant := uuid.New()
	dashboardExec(t, "INSERT INTO grants(id,principal_type,principal_id,principal_authority,organization_id,role) VALUES($1,'user',$2,'marble',$3,'ADMIN')", grant, user.String(), org)
	dashboardExec(t, "UPDATE grants SET revoked_at=now() WHERE id=$1", grant)

	after := dashboardRead(t, auth, 6)
	for _, entity := range dashboardEntities {
		require.Equal(t, before.Indicators[entity].Total, after.Indicators[entity].Total)
		require.Equal(t, *dashboardCurrent(t, before, entity).All, *dashboardCurrent(t, after, entity).All)
		require.Equal(t, *dashboardCurrent(t, before, entity).New, *dashboardCurrent(t, after, entity).New)
	}
	require.Equal(t, "renamed-"+tenant.String(), after.Indicators["tenants"].Recent[0].Name)
	require.Equal(t, "renamed-"+org.String(), after.Indicators["organizations"].Recent[0].Name)

	dashboardExec(t, "DELETE FROM grants WHERE id=$1", grant)
	dashboardExec(t, "DELETE FROM users WHERE id=$1", user)
	dashboardExec(t, "DELETE FROM organizations WHERE id=$1", org)
	dashboardExec(t, "DELETE FROM tenants WHERE id=$1", tenant)
}

func TestDashboardCoverageAndControlledHistory(t *testing.T) {
	auth := dashboardAuth(t)
	monday := dashboardMonday(time.Now())
	previousMonday := monday.AddDate(0, 0, -7)
	weekBefore := previousMonday.AddDate(0, 0, -7)
	tenant := uuid.New()
	dashboardSetCoverage(t, weekBefore)
	base := dashboardRead(t, auth, 6)
	dashboardExec(t, "INSERT INTO tenants(id,name) VALUES($1,$2)", tenant, tenant.String())
	dashboardHistoryFixture(t, func(exec func(string, ...any)) {
		exec(`UPDATE audit.audit_events SET created_at=$1 WHERE "table"='tenants' AND entity_id=$2`, previousMonday.Add(time.Hour), tenant)
	})
	d := dashboardRead(t, auth, 6)
	weeks := d.Indicators["tenants"].Weeks
	previous := dashboardWeek(t, d, "tenants", previousMonday)
	require.False(t, previous.Partial)
	require.NotNil(t, previous.All)
	require.Equal(t, *dashboardWeek(t, base, "tenants", previousMonday).All+1, *previous.All)
	require.NotNil(t, previous.New)
	require.Equal(t, *dashboardWeek(t, base, "tenants", previousMonday).New+1, *previous.New)
	before := dashboardWeek(t, d, "tenants", weekBefore)
	require.NotNil(t, before.New, "complete coverage without activity is a true zero")
	require.Equal(t, *dashboardWeek(t, base, "tenants", weekBefore).New, *before.New)
	require.Equal(t, *dashboardWeek(t, base, "tenants", weekBefore).All, *before.All)
	require.Nil(t, weeks[0].All, "history before coverage is a gap")
	require.Nil(t, weeks[0].New)
	// Deletion today reverses for the closed week and never removes its creation.
	dashboardExec(t, "UPDATE tenants SET deleted_at=now() WHERE id=$1", tenant)
	removed := dashboardRead(t, auth, 6)
	require.Equal(t, *previous.All, *dashboardWeek(t, removed, "tenants", previousMonday).All)
	require.Equal(t, *previous.New, *dashboardWeek(t, removed, "tenants", previousMonday).New)
	dashboardExec(t, "DELETE FROM tenants WHERE id=$1", tenant)

	// Coverage starting midweek counts only the covered part and flags the week.
	dashboardSetCoverage(t, previousMonday.AddDate(0, 0, 2))
	midweekBase := dashboardRead(t, auth, 6)
	covered, uncovered := uuid.New(), uuid.New()
	dashboardExec(t, "INSERT INTO tenants(id,name) VALUES($1,$2),($3,$4)", covered, covered.String(), uncovered, uncovered.String())
	dashboardHistoryFixture(t, func(exec func(string, ...any)) {
		exec(`UPDATE audit.audit_events SET created_at=$1 WHERE "table"='tenants' AND entity_id=$2`, previousMonday.AddDate(0, 0, 3), covered)
		exec(`UPDATE audit.audit_events SET created_at=$1 WHERE "table"='tenants' AND entity_id=$2`, previousMonday.AddDate(0, 0, 1), uncovered)
	})
	midweek := dashboardRead(t, auth, 6)
	partial := dashboardWeek(t, midweek, "tenants", previousMonday)
	require.True(t, partial.Partial)
	require.NotNil(t, partial.New)
	require.Equal(t, *dashboardWeek(t, midweekBase, "tenants", previousMonday).New+1, *partial.New)
	require.NotNil(t, partial.All, "the week-end observation is after coverage")
	gap := dashboardWeek(t, midweek, "tenants", weekBefore)
	require.Nil(t, gap.New)
	require.Nil(t, gap.All)
	require.False(t, gap.Partial)
	dashboardExec(t, "DELETE FROM tenants WHERE id=ANY($1)", []uuid.UUID{covered, uncovered})

	for _, months := range []int{1, 3, 6, 12} {
		dashboardSetCoverage(t, time.Now().AddDate(-2, 0, 0))
		ranged := dashboardRead(t, auth, months)
		require.Equal(t, months, ranged.Months)
		weeks := ranged.Indicators["users"].Weeks
		first := weeks[0]
		require.Equal(t, time.Monday, first.Start.Weekday())
		require.Equal(t, 0, first.Start.Hour())
		require.False(t, first.Start.After(ranged.RangeStart))
		require.Equal(t, first.Start.Before(ranged.RangeStart), first.Partial, "range-start week is partial unless aligned")
		require.True(t, weeks[len(weeks)-1].Partial, "current week is partial")
		require.WithinDuration(t, ranged.GeneratedAt, weeks[len(weeks)-1].End, time.Microsecond)
		for _, week := range weeks[1 : len(weeks)-1] {
			require.False(t, week.Partial)
			require.NotNil(t, week.New)
			require.NotNil(t, week.All)
		}
	}
}

// Events at the exact Monday boundary and across month and year transitions land
// in the Monday-starting UTC week that contains them.
func TestDashboardWeekBoundaries(t *testing.T) {
	auth := dashboardAuth(t)
	now := time.Now().UTC()
	monday := dashboardMonday(now)
	newYear := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	instants := []time.Time{
		monday.AddDate(0, 0, -7),
		monday.AddDate(0, 0, -7).Add(-time.Microsecond),
		newYear,
		newYear.Add(-time.Microsecond),
		firstOfMonth,
		firstOfMonth.Add(-time.Microsecond),
	}
	dashboardSetCoverage(t, now.AddDate(-2, 0, 0))
	before := dashboardRead(t, auth, 12)

	ids := make([]uuid.UUID, len(instants))
	for i := range instants {
		ids[i] = uuid.New()
		dashboardExec(t, "INSERT INTO tenants(id,name) VALUES($1,$2)", ids[i], ids[i].String())
	}
	dashboardHistoryFixture(t, func(exec func(string, ...any)) {
		for i, instant := range instants {
			exec(`UPDATE audit.audit_events SET created_at=$1 WHERE "table"='tenants' AND entity_id=$2`, instant, ids[i])
		}
	})
	after := dashboardRead(t, auth, 12)

	weeks := after.Indicators["tenants"].Weeks
	require.Len(t, weeks, len(before.Indicators["tenants"].Weeks))
	for i, week := range weeks {
		previous := before.Indicators["tenants"].Weeks[i]
		require.Equal(t, previous.Start, week.Start)
		expectedNew, expectedAll := 0, 0
		for _, instant := range instants {
			if dashboardMonday(instant).Equal(week.Start) {
				expectedNew++
			}
			if instant.Before(week.End) {
				expectedAll++
			}
		}
		require.Equal(t, *previous.New+expectedNew, *week.New, "new for week %s", week.Start)
		if i < len(weeks)-1 {
			require.Equal(t, *previous.All+expectedAll, *week.All, "all for week %s", week.Start)
		}
	}
	require.Equal(t, before.Indicators["tenants"].Total+len(instants), after.Indicators["tenants"].Total)
	dashboardExec(t, "DELETE FROM tenants WHERE id=ANY($1)", ids)
}

func TestDashboardRecentDatesAndLegacyDuplicates(t *testing.T) {
	auth := dashboardAuth(t)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		dashboardExec(t, "INSERT INTO users(id,email,first_name,role) VALUES($1,$2,$3,$4)", id, fmt.Sprintf("%s@dashboard.test", id), fmt.Sprintf("record-%d", i), models.VIEWER)
	}
	// Database clock: the dashboard snapshot excludes events after its own instant.
	var timestamp time.Time
	require.NoError(t, pgPool.QueryRow(context.Background(), "SELECT clock_timestamp()").Scan(&timestamp))
	dashboardHistoryFixture(t, func(exec func(string, ...any)) {
		exec(`UPDATE audit.audit_events SET created_at=$1 WHERE "table"='users' AND entity_id=ANY($2)`, timestamp, ids)
		exec(`DELETE FROM audit.audit_events WHERE "table"='users' AND entity_id=$1`, ids[3])
		// Exact legacy duplicate, different audit UUID; creation date is still one.
		exec(`INSERT INTO audit.audit_events(operation,"table",entity_id,data,created_at)
    SELECT operation,"table",entity_id,data,created_at FROM audit.audit_events WHERE "table"='users' AND entity_id=$1 AND operation='INSERT'`, ids[0])
	})
	dashboardExec(t, "UPDATE users SET first_name='current name' WHERE id=$1", ids[0])
	d := dashboardRead(t, auth, 6)
	require.True(t, d.Indicators["users"].RecentHasUnknown)
	require.Len(t, d.Indicators["users"].Recent, 3)
	for i, record := range d.Indicators["users"].Recent {
		require.NotEqual(t, ids[3], record.Id)
		require.WithinDuration(t, timestamp, record.CreatedAt, time.Microsecond)
		if i > 0 {
			require.Less(t, d.Indicators["users"].Recent[i-1].Id.String(), record.Id.String())
		}
		if record.Id == ids[0] {
			require.Equal(t, "current name", record.Name)
		}
	}
	dashboardExec(t, "DELETE FROM users WHERE id=ANY($1)", ids)
}

// The real import endpoint creates an organization and its admins in one actorless
// transaction; both are reflected in the metrics.
func TestDashboardOrganizationImport(t *testing.T) {
	auth := dashboardAuth(t)
	dashboardSetCoverage(t, dashboardMonday(time.Now()).AddDate(0, 0, -7))
	before := dashboardRead(t, auth, 6)
	name := "dashboard-import-" + uuid.NewString()
	auth.POST("/org-import").WithJSON(map[string]any{
		"org":    map[string]any{"name": name},
		"admins": []map[string]any{{"email": uuid.NewString() + "@dashboard.test", "role": "ADMIN", "first_name": "Imported", "last_name": "Admin"}},
	}).Expect().Status(http.StatusOK)
	after := dashboardRead(t, auth, 6)
	for _, entity := range []string{"organizations", "users"} {
		require.Equal(t, before.Indicators[entity].Total+1, after.Indicators[entity].Total)
		require.Equal(t, *dashboardCurrent(t, before, entity).New+1, *dashboardCurrent(t, after, entity).New)
	}
	require.Equal(t, name, after.Indicators["organizations"].Recent[0].Name)
	require.Equal(t, "Imported Admin", after.Indicators["users"].Recent[0].Name)
}

func TestDashboardTenantMerge(t *testing.T) {
	auth := dashboardAuth(t)
	dashboardSetCoverage(t, dashboardMonday(time.Now()).AddDate(0, 0, -14))
	before := dashboardRead(t, auth, 6)
	target := uuid.New()
	sources := []uuid.UUID{uuid.New(), uuid.New()}
	org := uuid.New()
	user := uuid.New()
	// Direct database creation in one actorless transaction is captured atomically.
	tx, err := pgPool.Begin(context.Background())
	require.NoError(t, err)
	for _, id := range append([]uuid.UUID{target}, sources...) {
		_, err = tx.Exec(context.Background(), "INSERT INTO tenants(id,name) VALUES($1,$2)", id, id.String())
		require.NoError(t, err)
	}
	_, err = tx.Exec(context.Background(), "INSERT INTO organizations(id,name,tenant_id) VALUES($1,$2,$3)", org, org.String(), sources[0])
	require.NoError(t, err)
	_, err = tx.Exec(context.Background(), "INSERT INTO users(id,email,organization_id,role) VALUES($1,$2,$3,$4)", user, user.String()+"@dashboard.test", org, models.ADMIN)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(context.Background()))
	created := dashboardRead(t, auth, 6)
	require.Equal(t, before.Indicators["tenants"].Total+3, created.Indicators["tenants"].Total)
	require.Equal(t, before.Indicators["organizations"].Total+1, created.Indicators["organizations"].Total)
	require.Equal(t, before.Indicators["users"].Total+1, created.Indicators["users"].Total)
	auth.POST("/tenants/{tenant_id}/merge", target.String()).WithJSON(map[string]any{"source_tenant_ids": sources, "name": "merged-" + target.String()}).Expect().Status(http.StatusNoContent)
	merged := dashboardRead(t, auth, 6)
	require.Equal(t, before.Indicators["tenants"].Total+1, merged.Indicators["tenants"].Total)
	require.Equal(t, before.Indicators["tenants"].Total+1, *dashboardCurrent(t, merged, "tenants").All)
	require.Equal(t, created.Indicators["organizations"].Total, merged.Indicators["organizations"].Total)
	require.Equal(t, created.Indicators["users"].Total, merged.Indicators["users"].Total)
	for _, entity := range dashboardEntities {
		require.Equal(t, *dashboardCurrent(t, created, entity).New, *dashboardCurrent(t, merged, entity).New)
	}
	auth.POST("/tenants/{tenant_id}/merge", target.String()).WithJSON(map[string]any{"source_tenant_ids": sources}).Expect().Status(http.StatusNoContent)
	repeated := dashboardRead(t, auth, 6)
	require.Equal(t, merged.Indicators["tenants"].Total, repeated.Indicators["tenants"].Total)
	require.Equal(t, *dashboardCurrent(t, merged, "tenants").All, *dashboardCurrent(t, repeated, "tenants").All)
	// Organization deletion does not imply deletion of surviving users or tenants.
	dashboardExec(t, "DELETE FROM organizations WHERE id=$1", org)
	surviving := dashboardRead(t, auth, 6)
	require.Equal(t, merged.Indicators["users"].Total, surviving.Indicators["users"].Total)
	require.Equal(t, merged.Indicators["tenants"].Total, surviving.Indicators["tenants"].Total)
	dashboardExec(t, "DELETE FROM users WHERE id=$1", user)
	dashboardExec(t, "DELETE FROM tenants WHERE id=ANY($1)", append([]uuid.UUID{target}, sources...))
}

func TestDashboardWeeksUseUTCInNonUTCDatabaseSession(t *testing.T) {
	ctx := context.Background()
	exec, release, err := testUsecases.NewExecutorFactory().NewPinnedExecutor(ctx)
	require.NoError(t, err)
	defer release()
	_, err = exec.Exec(ctx, "SET TIME ZONE 'America/New_York'")
	require.NoError(t, err)
	defer exec.Exec(ctx, "SET TIME ZONE 'UTC'") //nolint:errcheck
	result, err := testUsecases.Repositories.MarbleDbRepository.Dashboard(ctx, exec, 12)
	require.NoError(t, err)
	weeks := result.Indicators["organizations"].Weeks
	for i, week := range weeks {
		require.Equal(t, time.Monday, week.Start.Weekday())
		require.Equal(t, 0, week.Start.UTC().Hour())
		if i > 0 {
			require.Equal(t, 7*24*time.Hour, week.Start.Sub(weeks[i-1].Start))
		}
	}
}

func TestDashboardLegacyDuplicateCreationCount(t *testing.T) {
	auth := dashboardAuth(t)
	dashboardSetCoverage(t, dashboardMonday(time.Now()))
	before := dashboardRead(t, auth, 6)
	user := uuid.New()
	dashboardExec(t, "INSERT INTO users(id,email,role) VALUES($1,$2,$3)", user, user.String()+"@dashboard.test", models.VIEWER)
	dashboardExec(t, `INSERT INTO audit.audit_events(operation,"table",entity_id,data,created_at)
   SELECT operation,"table",entity_id,data,created_at FROM audit.audit_events
   WHERE "table"='users' AND entity_id=$1 AND operation='INSERT'`, user)
	d := dashboardRead(t, auth, 6)
	require.Equal(t, *dashboardCurrent(t, before, "users").New+1, *dashboardCurrent(t, d, "users").New)
	require.Equal(t, before.Indicators["users"].Total+1, d.Indicators["users"].Total)
	dashboardExec(t, "DELETE FROM users WHERE id=$1", user)
}

// Pruning or rewriting audit events invalidates only the intervals that lost
// events, only for the affected entity and measure, and never because of other
// audited tables.
func TestDashboardAuditPruningNarrowsCoverage(t *testing.T) {
	ctx := context.Background()
	start := time.Now().UTC().AddDate(0, 0, -30).Truncate(time.Microsecond)
	dashboardSetCoverage(t, start)
	initial := dashboardCoverageRows(t)

	// Unrelated or empty audit mutations keep every proof.
	unrelated := uuid.New()
	dashboardExec(t, `INSERT INTO audit.audit_events(operation,"table",entity_id,data,created_at) VALUES('UPDATE','decisions',$1,'{}',now())`, unrelated)
	dashboardExec(t, `UPDATE audit.audit_events SET data='{"edited":true}' WHERE entity_id=$1`, unrelated)
	dashboardExec(t, `DELETE FROM audit.audit_events WHERE entity_id=$1`, unrelated)
	dashboardExec(t, `DELETE FROM audit.audit_events WHERE false`)
	require.Equal(t, initial, dashboardCoverageRows(t))

	tenant := uuid.New()
	dashboardExec(t, "INSERT INTO tenants(id,name) VALUES($1,$2)", tenant, tenant.String())
	dashboardExec(t, "UPDATE tenants SET name='renamed' WHERE id=$1", tenant)

	// Re-scoping an event does not alter what the dashboard reads.
	dashboardExec(t, `UPDATE audit.audit_events SET org_id=$2 WHERE entity_id=$1`, tenant, uuid.New())
	require.Equal(t, initial, dashboardCoverageRows(t))

	// Pruning events older than coverage changes nothing.
	old := start.Add(-time.Hour)
	dashboardHistoryFixture(t, func(exec func(string, ...any)) {
		exec(`UPDATE audit.audit_events SET created_at=$2 WHERE entity_id=$1 AND operation='UPDATE'`, tenant, old)
	})
	dashboardExec(t, `DELETE FROM audit.audit_events WHERE entity_id=$1 AND operation='UPDATE'`, tenant)
	require.Equal(t, initial, dashboardCoverageRows(t))

	// Losing a covered creation moves tenants coverage just past it, both measures.
	var createdAt time.Time
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT created_at FROM audit.audit_events WHERE entity_id=$1 AND operation='INSERT'`, tenant).Scan(&createdAt))
	dashboardExec(t, `DELETE FROM audit.audit_events WHERE entity_id=$1 AND operation='INSERT'`, tenant)
	pruned := dashboardCoverageRows(t)
	require.Equal(t, createdAt.Add(time.Microsecond), pruned["tenants/all"])
	require.Equal(t, createdAt.Add(time.Microsecond), pruned["tenants/new"])
	for _, key := range []string{"organizations/all", "organizations/new", "users/all", "users/new"} {
		require.Equal(t, initial[key], pruned[key])
	}

	// Losing a non-creation event only invalidates All.
	user := uuid.New()
	dashboardExec(t, "INSERT INTO users(id,email,role) VALUES($1,$2,$3)", user, user.String()+"@dashboard.test", models.VIEWER)
	dashboardExec(t, "UPDATE users SET deleted_at=now() WHERE id=$1", user)
	var deletedAt time.Time
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT created_at FROM audit.audit_events WHERE entity_id=$1 AND operation='UPDATE'`, user).Scan(&deletedAt))
	dashboardExec(t, `DELETE FROM audit.audit_events WHERE entity_id=$1 AND operation='UPDATE'`, user)
	users := dashboardCoverageRows(t)
	require.Equal(t, deletedAt.Add(time.Microsecond), users["users/all"])
	require.Equal(t, initial["users/new"], users["users/new"])

	dashboardExec(t, "DELETE FROM users WHERE id=$1", user)
	dashboardExec(t, "DELETE FROM tenants WHERE id=$1", tenant)
}

func TestDashboardConcurrentPruning(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		dashboardExec(t, "INSERT INTO tenants(id,name) VALUES($1,$2)", id, id.String())
	}
	done := make(chan error, len(ids))
	for _, id := range ids {
		go func(id uuid.UUID) {
			_, err := pgPool.Exec(context.Background(), "DELETE FROM audit.audit_events WHERE entity_id=$1", id)
			done <- err
		}(id)
	}
	for range ids {
		require.NoError(t, <-done)
	}
	require.Len(t, dashboardCoverageRows(t), 6)
	dashboardExec(t, "DELETE FROM tenants WHERE id=ANY($1)", ids)
}

func TestDashboardTruncateInvalidatesCoverageTransactionally(t *testing.T) {
	auth := dashboardAuth(t)
	dashboardSetCoverage(t, dashboardMonday(time.Now()).AddDate(0, 0, -14))
	before := dashboardRead(t, auth, 6)
	ctx := context.Background()
	tx, err := testUsecases.NewExecutorFactory().NewExecutor().Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx) //nolint:errcheck
	_, err = tx.Exec(ctx, "TRUNCATE tenants CASCADE")
	require.NoError(t, err)
	inside, err := testUsecases.Repositories.MarbleDbRepository.Dashboard(ctx, tx, 6)
	require.NoError(t, err)
	tenants := inside.Indicators["tenants"]
	require.Zero(t, tenants.Total)
	require.True(t, tenants.Coverage.AllSince.After(before.GeneratedAt))
	require.True(t, before.Indicators["tenants"].Coverage.NewSince.Equal(*tenants.Coverage.NewSince), "recorded creations stay valid")
	require.Nil(t, tenants.Weeks[len(tenants.Weeks)-2].All)
	require.NotNil(t, tenants.Weeks[len(tenants.Weeks)-2].New)
	require.NoError(t, tx.Rollback(ctx))
	after := dashboardRead(t, auth, 6)
	require.Equal(t, before.Indicators["tenants"].Total, after.Indicators["tenants"].Total)
	require.Equal(t, before.Indicators["tenants"].Coverage, after.Indicators["tenants"].Coverage)
}

// Organization-scoped audit consumers keep seeing exactly the actor-driven events
// they saw before; actorless lifecycle events stay out of their scope.
func TestDashboardAuditScopeForOrganizationAdmins(t *testing.T) {
	ctx := context.Background()
	tenant, org, user := uuid.New(), uuid.New(), uuid.New()
	// Use the application's transaction seam, which clears pooled actor context.
	err := testUsecases.NewTransactionFactory().Transaction(ctx, func(tx repositories.Transaction) error {
		if _, err := tx.Exec(ctx, "INSERT INTO tenants(id,name) VALUES($1,$2)", tenant, tenant.String()); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "INSERT INTO organizations(id,name,tenant_id) VALUES($1,$2,$3)", org, org.String(), tenant)
		return err
	})
	require.NoError(t, err)
	email := user.String() + "@dashboard.test"
	dashboardExec(t, "INSERT INTO users(id,email,organization_id,role) VALUES($1,$2,$3,$4)", user, email, org, models.ADMIN)
	dashboardExec(t, "INSERT INTO grants(id,principal_type,principal_id,principal_authority,organization_id,role) VALUES($1,'user',$2,'marble',$3,'ADMIN')", uuid.New(), user.String(), org)

	var unscoped int
	require.NoError(t, pgPool.QueryRow(ctx, `SELECT count(*) FROM audit.audit_events WHERE entity_id = ANY($1) AND org_id IS NULL AND tenant_id IS NULL`, []uuid.UUID{tenant, org, user}).Scan(&unscoped))
	require.Equal(t, 3, unscoped)

	// An actor-driven change is scoped from the session, as with global_audit.
	actorCtx := context.WithValue(ctx, utils.ContextKeyCredentials, models.Credentials{
		ActorIdentity:  models.Identity{UserId: models.UserId(user.String())},
		OrganizationId: org,
	})
	_, err = testUsecases.NewExecutorFactory().NewExecutor().Exec(actorCtx, "UPDATE users SET first_name='edited' WHERE id=$1", user)
	require.NoError(t, err)

	e := httpexpect.Default(t, testServer.URL)
	token := e.POST("/token").WithHeader("Authorization", "Bearer "+firebaseDummyToken(email)).Expect().Status(http.StatusOK).JSON().Object().Value("access_token").String().Raw()
	read := func(table, entity string) *httpexpect.Array {
		return e.GET("/admin/audit-events").WithHeader("Authorization", "Bearer "+token).
			WithQuery("table", table).WithQuery("entity_id", entity).
			Expect().Status(http.StatusOK).JSON().Object().Value("events").Array()
	}
	read("organizations", org.String()).IsEmpty()
	users := read("users", user.String())
	users.Length().IsEqual(1)
	users.Value(0).Object().Value("operation").String().IsEqual("UPDATE")
	users.Value(0).Object().Value("actor").Object().Value("type").String().IsEqual("user")

	dashboardExec(t, "DELETE FROM users WHERE id=$1", user)
	dashboardExec(t, "DELETE FROM grants WHERE organization_id=$1", org)
	dashboardExec(t, "DELETE FROM organizations WHERE id=$1", org)
	dashboardExec(t, "DELETE FROM tenants WHERE id=$1", tenant)
}

// Licences are never deleted and keep their insert-time created_at, so their history
// is complete without audit coverage: no gaps, edits never change counts.
func TestDashboardLicenses(t *testing.T) {
	auth := dashboardAuth(t)
	dashboardSetCoverage(t, time.Now()) // audit coverage must not affect licences
	before := dashboardRead(t, auth, 3)
	name := "dashboard-licence-" + uuid.NewString()
	id := auth.POST("/licenses").WithJSON(map[string]any{
		"expiration_date":      time.Now().AddDate(1, 0, 0).UTC().Format(time.RFC3339),
		"organization_name":    name,
		"description":          "dashboard test",
		"license_entitlements": map[string]any{},
	}).Expect().Status(http.StatusOK).JSON().Object().Value("license").Object().Value("id").String().Raw()

	created := dashboardRead(t, auth, 3)
	licences := created.Indicators["licenses"]
	require.Equal(t, before.Indicators["licenses"].Total+1, licences.Total)
	require.Equal(t, *dashboardCurrent(t, before, "licenses").New+1, *dashboardCurrent(t, created, "licenses").New)
	require.Equal(t, name, licences.Recent[0].Name)
	require.False(t, licences.RecentHasUnknown)
	require.True(t, licences.Coverage.AllSince.Equal(created.RangeStart))
	for _, week := range licences.Weeks {
		require.NotNil(t, week.All, "licence history has no gaps")
		require.NotNil(t, week.New)
	}

	// Backdated creation moves into its week; suspension and edits keep the listed count.
	previousMonday := dashboardMonday(time.Now()).AddDate(0, 0, -7)
	dashboardExec(t, "UPDATE licenses SET created_at=$2 WHERE id=$1", id, previousMonday.Add(time.Hour))
	auth.PATCH("/licenses/{license_id}", id).WithJSON(map[string]any{"suspend": true, "description": "edited"}).Expect().Status(http.StatusOK)
	backdated := dashboardRead(t, auth, 3)
	require.Equal(t, licences.Total, backdated.Indicators["licenses"].Total)
	require.Equal(t, *dashboardCurrent(t, before, "licenses").New, *dashboardCurrent(t, backdated, "licenses").New)
	require.Equal(t, *dashboardWeek(t, before, "licenses", previousMonday).New+1, *dashboardWeek(t, backdated, "licenses", previousMonday).New)
	require.Equal(t, *dashboardWeek(t, before, "licenses", previousMonday).All+1, *dashboardWeek(t, backdated, "licenses", previousMonday).All)
	require.Equal(t, *dashboardWeek(t, before, "licenses", previousMonday.AddDate(0, 0, -7)).All, *dashboardWeek(t, backdated, "licenses", previousMonday.AddDate(0, 0, -7)).All)
	dashboardExec(t, "DELETE FROM licenses WHERE id=$1", id)
}
