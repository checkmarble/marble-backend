package repositories

import (
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
)

func (repo *MarbleDbRepository) GetInboxById(tx Transaction, inboxId string) (models.Inbox, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(pgTx,
		selectInboxesJoinUsers().Where(squirrel.Eq{"i.id": inboxId}),
		dbmodels.AdaptInboxWithUsers,
	)
}

func (repo *MarbleDbRepository) ListInboxes(tx Transaction, organizationId string, inboxIds []string) ([]models.Inbox, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := selectInboxesJoinUsers().
		Where(squirrel.Eq{"i.organization_id": organizationId})

	if len(inboxIds) > 0 {
		query = query.Where(squirrel.Eq{"i.id": inboxIds})
	}

	return SqlToListOfModels(pgTx,
		query,
		dbmodels.AdaptInboxWithUsers,
	)
}

func selectInboxesJoinUsers() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(pure_utils.WithPrefix(dbmodels.SelectInboxColumn, "i")...).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY u.created_at) FILTER (WHERE u.id IS NOT NULL) as inbox_users",
				strings.Join(pure_utils.WithPrefix(dbmodels.SelectInboxUserColumn, "u"), ","),
			),
		).
		From(dbmodels.TABLE_INBOXES + " AS i").
		LeftJoin(dbmodels.TABLE_INBOX_USERS + " AS u ON u.inbox_id = i.id").
		GroupBy("i.id").
		OrderBy("i.created_at DESC")
}

func (repo *MarbleDbRepository) CreateInbox(tx Transaction, input models.CreateInboxInput, newInboxId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_INBOXES).
			Columns(
				"id",
				"organization_id",
				"name",
			).
			Values(
				newInboxId,
				input.OrganizationId,
				input.Name,
			),
	)
	return err
}

func (repo *MarbleDbRepository) GetInboxUserById(tx Transaction, inboxUserId string) (models.InboxUser, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectInboxUserColumn...).
			From(dbmodels.TABLE_INBOX_USERS).
			Where(squirrel.Eq{"id": inboxUserId}),
		dbmodels.AdaptInboxUser,
	)
}

func (repo *MarbleDbRepository) ListInboxUsers(tx Transaction, filters models.InboxUserFilterInput) ([]models.InboxUser, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().
		Select(dbmodels.SelectInboxUserColumn...).
		From(dbmodels.TABLE_INBOX_USERS)

	if filters.InboxId != "" {
		query = query.Where(squirrel.Eq{"inbox_id": filters.InboxId})
	} else if filters.UserId != "" {
		query = query.Where(squirrel.Eq{"user_id": filters.UserId})
	} else {
		return []models.InboxUser{}, errors.New("must define either inbox_id or user_id as a filter in repositories/ListInboxUsers")
	}

	return SqlToListOfModels(pgTx,
		query,
		dbmodels.AdaptInboxUser,
	)
}

func (repo *MarbleDbRepository) CreateInboxUser(tx Transaction, input models.CreateInboxUserInput, newInboxUserId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
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