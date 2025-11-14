package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"

	"github.com/Masterminds/squirrel"
)

const CASES_COUNT_LIMIT = 999

func (repo *MarbleDbRepository) GetInboxById(ctx context.Context, exec Executor, inboxId uuid.UUID) (models.Inbox, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Inbox{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectInboxesJoinUsers().Where(squirrel.Eq{"i.id": inboxId}),
		dbmodels.AdaptInboxWithUsers,
	)
}

func (repo *MarbleDbRepository) ListInboxes(ctx context.Context, exec Executor,
	organizationId string, inboxIds []uuid.UUID, withCaseCount bool,
) ([]models.Inbox, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectInboxesJoinUsers().
		Where(squirrel.Eq{"i.status": models.InboxStatusActive}).
		Where(squirrel.Eq{"i.organization_id": organizationId})

	if len(inboxIds) > 0 {
		query = query.Where(squirrel.Eq{"i.id": inboxIds})
	}

	// condition MUST be "status IN ('pending', 'investigating')" and not "status!='closed'" because of the index selection
	if withCaseCount {
		query = query.Column(
			fmt.Sprintf(`(
	SELECT count(*)
	FROM (
		SELECT 1
		FROM cases AS c
		WHERE c.org_id = i.organization_id
			AND c.inbox_id = i.id
			AND (status in ('pending', 'investigating'))
			AND (snoozed_until IS NULL OR snoozed_until < now())
		LIMIT %d
		) AS cases_count_inner
	) AS cases_count`,
				CASES_COUNT_LIMIT))
		return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptInboxWithCasesCount)
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptInboxWithUsers,
	)
}

func selectInboxesJoinUsers() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(columnsNames("i", dbmodels.SelectInboxColumn)...).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY u.created_at) FILTER (WHERE u.id IS NOT NULL) as inbox_users",
				strings.Join(columnsNames("u", dbmodels.SelectInboxUserColumn), ","),
			),
		).
		From(dbmodels.TABLE_INBOXES + " AS i").
		LeftJoin(dbmodels.TABLE_INBOX_USERS + " AS u ON u.inbox_id = i.id").
		GroupBy("i.id").
		OrderBy("i.created_at DESC")
}

func (repo *MarbleDbRepository) CreateInbox(
	ctx context.Context,
	exec Executor,
	input models.CreateInboxInput,
	newInboxId uuid.UUID,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_INBOXES).
			Columns(
				"id",
				"organization_id",
				"name",
				"escalation_inbox_id",
			).
			Values(
				newInboxId,
				input.OrganizationId,
				input.Name,
				input.EscalationInboxId,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateInbox(
	ctx context.Context,
	exec Executor,
	inboxId uuid.UUID,
	name *string,
	escalationInboxId *uuid.UUID,
	autoAssignEnabled *bool,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().Update(dbmodels.TABLE_INBOXES).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"id": inboxId})

	hasUpdates := false

	if name != nil {
		sql = sql.Set("name", *name)
		hasUpdates = true
	}
	if escalationInboxId != nil {
		sql = sql.Set("escalation_inbox_id", *escalationInboxId)
		hasUpdates = true
	}
	if autoAssignEnabled != nil {
		sql = sql.Set("auto_assign_enabled", *autoAssignEnabled)
		hasUpdates = true
	}

	if !hasUpdates {
		return nil
	}

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) SoftDeleteInbox(ctx context.Context, exec Executor, inboxId uuid.UUID) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Update(dbmodels.TABLE_INBOXES).
			Set("status", models.InboxStatusInactive).
			Set("updated_at", squirrel.Expr("NOW()")).
			Where(squirrel.Eq{"id": inboxId}),
	)
	return err
}
