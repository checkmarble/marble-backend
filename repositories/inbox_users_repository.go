package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

func selectInboxUsers() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(columnsNames("u", dbmodels.SelectInboxUserWithOrgIdColumn)...).
		Column("i.organization_id").
		From(dbmodels.TABLE_INBOX_USERS + " AS u").
		Join(dbmodels.TABLE_INBOXES + " AS i ON i.id = inbox_id")
}

func (repo *MarbleDbRepository) GetInboxUserById(ctx context.Context, exec Executor, inboxUserId string) (models.InboxUser, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.InboxUser{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectInboxUsers().
			Where(squirrel.Eq{"u.id": inboxUserId}),
		dbmodels.AdaptInboxUserWithOrgId,
	)
}

func (repo *MarbleDbRepository) ListInboxUsers(ctx context.Context, exec Executor, filters models.InboxUserFilterInput) ([]models.InboxUser, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectInboxUsers()

	if filters.InboxId != "" {
		query = query.Where(squirrel.Eq{"u.inbox_id": filters.InboxId})
	}
	if filters.UserId != "" {
		query = query.Where(squirrel.Eq{"u.user_id": filters.UserId})
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptInboxUserWithOrgId,
	)
}

func (repo *MarbleDbRepository) CreateInboxUser(ctx context.Context, exec Executor, input models.CreateInboxUserInput, newInboxUserId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_INBOX_USERS).
			Columns(
				"id",
				"inbox_id",
				"user_id",
				"role",
			).
			Values(
				newInboxUserId,
				input.InboxId,
				input.UserId,
				input.Role,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateInboxUser(ctx context.Context, exec Executor, inboxUserId string, role models.InboxUserRole) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Update(dbmodels.TABLE_INBOX_USERS).
			Set("role", role).
			Set("updated_at", "NOW()").
			Where(squirrel.Eq{"id": inboxUserId}),
	)
	return err
}

func (repo *MarbleDbRepository) DeleteInboxUser(ctx context.Context, exec Executor, inboxUserId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().Delete(dbmodels.TABLE_INBOX_USERS).
			Where(squirrel.Eq{"id": inboxUserId}),
	)
	return err
}
