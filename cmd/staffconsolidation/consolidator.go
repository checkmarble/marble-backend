package staffconsolidation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var aliasPattern = regexp.MustCompile(`^([^+@]+)\+[^@]+@checkmarble\.com$`)

type Config struct {
	MarblePool     *pgxpool.Pool
	ClientExecutor func(context.Context, uuid.UUID) (repositories.Executor, error)
	FirebaseAdmin  FirebaseAdmin
	Logger         *slog.Logger
}

type FirebaseAdmin interface {
	EnsureUser(ctx context.Context, email, name string) (bool, error)
}

const metadataKey = "staff_accounts_consolidated"
const advisoryLockCleanupTimeout = 5 * time.Second

func RunOnce(ctx context.Context, cfg Config) error {
	if cfg.MarblePool == nil {
		return fmt.Errorf("marble database pool is required")
	}
	conn, err := cfg.MarblePool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, metadataKey); err != nil {
		return err
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), advisoryLockCleanupTimeout)
		_, unlockErr := conn.Exec(unlockCtx, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, metadataKey)
		cancel()
		if unlockErr == nil {
			return
		}

		logInfo(cfg.Logger, "failed to release staff consolidation lock", "error", unlockErr)
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), advisoryLockCleanupTimeout)
		closeErr := conn.Hijack().Close(closeCtx)
		cancel()
		if closeErr != nil {
			logInfo(cfg.Logger, "failed to close staff consolidation connection", "error", closeErr)
		}
	}()

	var value string
	err = conn.QueryRow(ctx, `SELECT value FROM metadata WHERE org_id IS NULL AND key = $1`, metadataKey).Scan(&value)
	if err == nil && value == "true" {
		logInfo(cfg.Logger, "staff consolidation already completed", "metadata_key", metadataKey)
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	report, err := Discover(ctx, cfg.MarblePool)
	if err != nil {
		return err
	}
	logInfo(cfg.Logger, "staff consolidation discovered accounts", "canonical_accounts", len(report.Groups))
	if _, err := Apply(ctx, cfg, report); err != nil {
		return err
	}
	_, err = conn.Exec(ctx, `
		INSERT INTO metadata (key, value) VALUES ($1, 'true')
		ON CONFLICT (org_id, key) DO UPDATE SET value = EXCLUDED.value`, metadataKey)
	if err != nil {
		return err
	}
	logInfo(cfg.Logger, "staff consolidation completed", "metadata_key", metadataKey)
	return nil
}

type Report struct {
	Groups []GroupReport
}

type GroupReport struct {
	CanonicalEmail  string
	CanonicalID     string
	CanonicalReason string
	Skipped         bool
	SkipReason      string
	AliasIDs        []string
	AliasUUIDs      []uuid.UUID
	Organizations   []uuid.UUID
	HasActiveAlias  bool
}

type userRow struct {
	id      string
	email   string
	role    int
	orgID   *uuid.UUID
	deleted bool
}

func Discover(ctx context.Context, pool *pgxpool.Pool) (Report, error) {
	rows, err := pool.Query(ctx, `
		SELECT id::text, email, role, organization_id, deleted_at IS NOT NULL
		FROM users
		WHERE lower(email) LIKE '%@checkmarble.com'
		ORDER BY lower(email), id`)
	if err != nil {
		return Report{}, err
	}
	defer rows.Close()

	membersByCanonical := make(map[string][]userRow)
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.id, &u.email, &u.role, &u.orgID, &u.deleted); err != nil {
			return Report{}, err
		}
		if canonical, ok := canonicalEmail(u.email); ok {
			membersByCanonical[canonical] = append(membersByCanonical[canonical], u)
		} else if strings.HasSuffix(strings.ToLower(u.email), "@checkmarble.com") {
			membersByCanonical[strings.ToLower(u.email)] = append(membersByCanonical[strings.ToLower(u.email)], u)
		}
	}
	if err := rows.Err(); err != nil {
		return Report{}, err
	}

	keys := make([]string, 0, len(membersByCanonical))
	for key, members := range membersByCanonical {
		hasAlias := false
		for _, member := range members {
			hasAlias = hasAlias || aliasPattern.MatchString(strings.ToLower(member.email))
		}
		if !hasAlias {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	report := Report{Groups: make([]GroupReport, 0, len(keys))}
	for _, email := range keys {
		members := membersByCanonical[email]
		hasActiveAlias := false
		for _, member := range members {
			hasActiveAlias = hasActiveAlias || (aliasPattern.MatchString(strings.ToLower(member.email)) && !member.deleted)
		}
		if !hasActiveAlias {
			group := GroupReport{
				CanonicalEmail: strings.ToLower(email),
				Skipped:        true,
				SkipReason:     "all_aliases_deleted",
			}
			for _, member := range members {
				if aliasPattern.MatchString(strings.ToLower(member.email)) {
					group.AliasIDs = append(group.AliasIDs, member.id)
				}
			}
			report.Groups = append(report.Groups, group)
			continue
		}
		plain := make([]userRow, 0)
		for _, member := range members {
			if strings.EqualFold(member.email, email) && !member.deleted {
				plain = append(plain, member)
			}
		}
		var canonical userRow
		reason := "first_alias"
		if len(plain) > 0 {
			sort.Slice(plain, func(i, j int) bool { return plain[i].id < plain[j].id })
			canonical = plain[0]
			reason = "plain_email"
		} else {
			canonical, _ = chooseAliasCanonical(members)
		}
		group := GroupReport{
			CanonicalEmail:  strings.ToLower(email),
			CanonicalID:     canonical.id,
			CanonicalReason: reason,
			HasActiveAlias:  !canonical.deleted,
		}
		orgs := make(map[uuid.UUID]struct{})
		for _, alias := range members {
			if alias.id == canonical.id {
				continue
			}
			group.AliasIDs = append(group.AliasIDs, alias.id)
			aliasID, err := uuid.Parse(alias.id)
			if err != nil {
				return Report{}, fmt.Errorf("alias user %q has invalid UUID: %w", alias.email, err)
			}
			group.AliasUUIDs = append(group.AliasUUIDs, aliasID)
			group.HasActiveAlias = group.HasActiveAlias || !alias.deleted
			if alias.orgID != nil && *alias.orgID != uuid.Nil {
				orgs[*alias.orgID] = struct{}{}
			}
		}
		for orgID := range orgs {
			group.Organizations = append(group.Organizations, orgID)
		}
		sort.Slice(group.Organizations, func(i, j int) bool { return group.Organizations[i].String() < group.Organizations[j].String() })
		report.Groups = append(report.Groups, group)
	}
	return report, nil
}

func chooseAliasCanonical(members []userRow) (userRow, bool) {
	active := make([]userRow, 0, len(members))
	all := make([]userRow, 0, len(members))
	for _, member := range members {
		if !aliasPattern.MatchString(strings.ToLower(member.email)) {
			continue
		}
		all = append(all, member)
		if !member.deleted {
			active = append(active, member)
		}
	}
	candidates := active
	if len(candidates) == 0 {
		candidates = all
	}
	if len(candidates) == 0 {
		return userRow{}, false
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].id < candidates[j].id })
	return candidates[0], true
}

func canonicalEmail(email string) (string, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	match := aliasPattern.FindStringSubmatch(email)
	if match == nil {
		return "", false
	}
	return match[1] + "@checkmarble.com", true
}

func Apply(ctx context.Context, cfg Config, report Report) (Report, error) {
	if cfg.MarblePool == nil {
		return Report{}, fmt.Errorf("marble database pool is required")
	}
	var result = report
	for _, group := range report.Groups {
		if group.Skipped {
			continue
		}
		for _, organizationID := range group.Organizations {
			if err := applyClientAudit(ctx, cfg, organizationID, group); err != nil {
				return result, fmt.Errorf("consolidate %s in organization %s: %w", group.CanonicalEmail, organizationID, err)
			}
		}
	}
	err := pgx.BeginFunc(ctx, cfg.MarblePool, func(tx pgx.Tx) error {
		for _, group := range report.Groups {
			if group.Skipped {
				logInfo(cfg.Logger, "staff consolidation group skipped", "canonical_email", group.CanonicalEmail, "reason", group.SkipReason, "aliases", len(group.AliasIDs))
				continue
			}
			logGroup(cfg.Logger, group)
			if group.HasActiveAlias {
				if cfg.FirebaseAdmin == nil {
					logInfo(cfg.Logger, "staff consolidation firebase reset skipped", "canonical_email", group.CanonicalEmail, "reason", "non-firebase authentication")
				} else {
					created, err := cfg.FirebaseAdmin.EnsureUser(ctx, group.CanonicalEmail, group.CanonicalEmail)
					if err != nil {
						return fmt.Errorf("firebase setup for %s: %w", group.CanonicalEmail, err)
					}
					if created {
						logInfo(cfg.Logger, "staff consolidation firebase user created and reset sent", "canonical_email", group.CanonicalEmail)
					} else {
						logInfo(cfg.Logger, "staff consolidation firebase user already exists; reset skipped", "canonical_email", group.CanonicalEmail)
					}
				}
			}
			if err := applyGroup(ctx, tx, cfg, group); err != nil {
				return fmt.Errorf("consolidate %s: %w", group.CanonicalEmail, err)
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func applyGroup(ctx context.Context, tx pgx.Tx, cfg Config, group GroupReport) error {
	if _, err := tx.Exec(ctx, `UPDATE users SET email = $1 WHERE id = $2::uuid`, group.CanonicalEmail, group.CanonicalID); err != nil {
		return err
	}

	grantTag, err := tx.Exec(ctx, `
			INSERT INTO grants (id, principal_type, principal_id, principal_authority, tenant_id, organization_id, role, created_at, expires_at)
			SELECT uuid_generate_v4(), 'user', $1, g.principal_authority, g.tenant_id, g.organization_id, g.role, g.created_at, g.expires_at
			FROM active_grants g
			WHERE g.principal_type = 'user' AND g.principal_id = ANY($2::text[])
			ON CONFLICT DO NOTHING`, group.CanonicalID, group.AliasIDs)
	if err != nil {
		return err
	}
	logInfo(cfg.Logger, "staff consolidation grants created", "canonical_email", group.CanonicalEmail, "rows", grantTag.RowsAffected())

	for _, table := range []string{"inbox_users", "case_contributors"} {
		column := "inbox_id"
		if table == "case_contributors" {
			column = "case_id"
		}
		deletedTag, err := tx.Exec(ctx, fmt.Sprintf(`
				DELETE FROM %s a
				WHERE a.user_id = ANY($1::uuid[])
				  AND EXISTS (
					SELECT 1 FROM %s b
					WHERE a.%s = b.%s
					  AND (b.user_id = $2::uuid OR (b.user_id = ANY($1::uuid[]) AND b.ctid > a.ctid))
			  )`, table, table, column, column), group.AliasUUIDs, group.CanonicalID)
		if err != nil {
			return err
		}
		logInfo(cfg.Logger, "staff consolidation duplicate memberships removed", "canonical_email", group.CanonicalEmail, "table", table, "rows", deletedTag.RowsAffected())
	}

	for _, ref := range []struct{ table, column string }{
		{"inbox_users", "user_id"}, {"case_contributors", "user_id"},
		{"rule_snoozes", "created_by_user"}, {"cases", "assigned_to"},
		{"screenings", "requested_by"}, {"screening_matches", "reviewed_by"},
		{"screening_match_comments", "commented_by"}, {"screening_whitelists", "whitelisted_by"},
		{"entity_annotations", "annotated_by"}, {"suspicious_activity_reports", "created_by"},
		{"suspicious_activity_reports", "uploaded_by"}, {"user_unavailabilities", "user_id"},
		{"continuous_screening_matches", "reviewed_by"}, {"screening_freeform_searches", "user_id"},
		{"upload_logs", "user_id"}, {"scoring_scores", "overridden_by"},
	} {
		updatedTag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET %s = $1 WHERE %s = ANY($2::uuid[])`, ref.table, ref.column, ref.column), group.CanonicalID, group.AliasUUIDs)
		if err != nil {
			return err
		}
		logInfo(cfg.Logger, "staff consolidation references repointed", "canonical_email", group.CanonicalEmail, "table", ref.table, "column", ref.column, "rows", updatedTag.RowsAffected())
	}
	caseUserTag, err := tx.Exec(ctx, `UPDATE case_events SET user_id = $1 WHERE user_id = ANY($2::uuid[])`, group.CanonicalID, group.AliasUUIDs)
	if err != nil {
		return err
	}
	logInfo(cfg.Logger, "staff consolidation case event users repointed", "canonical_email", group.CanonicalEmail, "rows", caseUserTag.RowsAffected())
	caseValueTag, err := tx.Exec(ctx, `UPDATE case_events SET new_value = $1 WHERE event_type = 'case_assigned' AND new_value = ANY($2::text[])`, group.CanonicalID, group.AliasIDs)
	if err != nil {
		return err
	}
	logInfo(cfg.Logger, "staff consolidation case assignment values repointed", "canonical_email", group.CanonicalEmail, "rows", caseValueTag.RowsAffected())
	deletedUsersTag, err := tx.Exec(ctx, `UPDATE users SET deleted_at = COALESCE(deleted_at, NOW()) WHERE id = ANY($1::uuid[])`, group.AliasUUIDs)
	if err != nil {
		return err
	}
	logInfo(cfg.Logger, "staff consolidation aliases soft-deleted", "canonical_email", group.CanonicalEmail, "rows", deletedUsersTag.RowsAffected())
	revokedTag, err := tx.Exec(ctx, `UPDATE grants SET revoked_at = COALESCE(revoked_at, NOW()) WHERE principal_type = 'user' AND principal_id = ANY($1::text[]) AND revoked_at IS NULL`, group.AliasIDs)
	if err != nil {
		return err
	}
	logInfo(cfg.Logger, "staff consolidation alias grants revoked", "canonical_email", group.CanonicalEmail, "rows", revokedTag.RowsAffected())
	return err
}

func logGroup(logger *slog.Logger, group GroupReport) {
	logInfo(logger, "staff consolidation canonical selected",
		"canonical_email", group.CanonicalEmail,
		"canonical_id", group.CanonicalID,
		"reason", group.CanonicalReason,
		"aliases_to_merge", len(group.AliasIDs),
		"organizations", len(group.Organizations))
}

func logInfo(logger *slog.Logger, message string, args ...any) {
	if logger != nil {
		logger.Info(message, args...)
	}
}

func applyClientAudit(ctx context.Context, cfg Config, organizationID uuid.UUID, group GroupReport) error {
	if cfg.ClientExecutor == nil {
		return fmt.Errorf("client database executor is not configured")
	}
	exec, err := cfg.ClientExecutor(ctx, organizationID)
	if err != nil {
		return err
	}
	tx, err := exec.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	schema := exec.DatabaseSchema().Schema
	table := pgx.Identifier{schema, "_monitored_objects_audit"}.Sanitize()
	if err := func() error {
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = $1 AND table_name = '_monitored_objects_audit'
			)`, schema).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			logInfo(cfg.Logger, "staff consolidation client audit table absent", "organization_id", organizationID)
			return nil
		}
		updatedTag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET user_id = $1 WHERE user_id = ANY($2::uuid[])`, table), group.CanonicalID, group.AliasUUIDs)
		if err == nil {
			logInfo(cfg.Logger, "staff consolidation client audit references repointed", "canonical_email", group.CanonicalEmail, "organization_id", organizationID, "rows", updatedTag.RowsAffected())
		}
		return err
	}(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
