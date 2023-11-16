package repositories

import (
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) ListCaseEvents(tx Transaction, caseId string) ([]models.CaseEvent, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := selectJoinCaseEventAndUser().Where(squirrel.Eq{"case_id": caseId})

	return SqlToListOfRow(
		pgTx,
		query,
		adaptCaseEventAndUser,
	)
}

type dbJoinCaseEventAndUser struct {
	dbmodels.DBCaseEvent
	dbmodels.DBUserResult
}

func selectJoinCaseEventAndUser() squirrel.SelectBuilder {
	var columns []string
	columns = append(columns, columnsNames("ce", dbmodels.SelectCaseEventColumn)...)
	columns = append(columns, columnsNames("u", dbmodels.UserFields)...)
	return NewQueryBuilder().
		Select(columns...).
		From(fmt.Sprintf("%s AS ce", dbmodels.TABLE_CASE_EVENTS)).
		Join(fmt.Sprintf("%s AS u ON u.id = ce.user_id", dbmodels.TABLE_USERS)).
		OrderBy("ce.created_at DESC")
}

func adaptCaseEventAndUser(row pgx.CollectableRow) (models.CaseEvent, error) {
	db, err := pgx.RowToStructByPos[dbJoinCaseEventAndUser](row)
	if err != nil {
		return models.CaseEvent{}, err
	}

	user, err := dbmodels.AdaptUser(db.DBUserResult)
	if err != nil {
		return models.CaseEvent{}, err
	}

	return dbmodels.AdaptCaseEvent(db.DBCaseEvent, user), nil
}

func (repo *MarbleDbRepository) CreateCaseEvent(tx Transaction, createCaseEventAttributes models.CreateCaseEventAttributes) error {
	return repo.BatchCreateCaseEvents(tx, []models.CreateCaseEventAttributes{createCaseEventAttributes})
}

func (repo *MarbleDbRepository) BatchCreateCaseEvents(tx Transaction, createCaseEventAttributes []models.CreateCaseEventAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Insert(dbmodels.TABLE_CASE_EVENTS).
		Columns(
			"id",
			"case_id",
			"user_id",
			"event_type",
			"resource_id",
			"resource_type",
			"new_value",
			"previous_value",
		)

	for _, createCaseEventAttribute := range createCaseEventAttributes {
		query = query.Values(
			uuid.NewString(),
			createCaseEventAttribute.CaseId,
			createCaseEventAttribute.UserId,
			createCaseEventAttribute.EventType,
			createCaseEventAttribute.ResourceId,
			createCaseEventAttribute.ResourceType,
			createCaseEventAttribute.NewValue,
			createCaseEventAttribute.PreviousValue,
		)
	}

	_, err := pgTx.ExecBuilder(query)

	return err
}
