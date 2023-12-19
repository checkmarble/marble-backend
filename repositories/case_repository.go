package repositories

import (
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) ListOrganizationCases(
	tx Transaction,
	filters models.CaseFilters,
	pagination models.PaginationAndSorting,
) ([]models.CaseWithRank, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	coreQuery := casesCoreQueryWithRank(pagination)
	filteredCoreQuery := applyCaseFilters(coreQuery, filters)

	subquery := squirrel.StatementBuilder.
		Select(casesWithRankColumns()...).
		FromSelect(filteredCoreQuery, "s").
		Limit(uint64(pagination.Limit))

	paginatedSubquery, err := applyCasesPagination(subquery, pagination)
	if err != nil {
		return []models.CaseWithRank{}, err
	}
	queryWithJoinedFields := selectCasesWithJoinedFields(paginatedSubquery, pagination, true)

	return SqlToListOfRow(pgTx, queryWithJoinedFields, func(row pgx.CollectableRow) (models.CaseWithRank, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DBPaginatedCases](row)
		if err != nil {
			return models.CaseWithRank{}, err
		}

		return dbmodels.AdaptCaseWithRank(db.DBCaseWithContributorsAndTags, db.RankNumber, db.Total)
	})
}

func (repo *MarbleDbRepository) GetCaseById(tx Transaction, caseId string) (models.Case, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		selectCasesWithJoinedFields(squirrel.SelectBuilder(NewQueryBuilder()), models.PaginationAndSorting{Sorting: models.CasesSortingCreatedAt}, false).
			Where(squirrel.Eq{"c.id": caseId}),
		dbmodels.AdaptCaseWithContributorsAndTags,
	)
}

func (repo *MarbleDbRepository) CreateCase(tx Transaction, createCaseAttributes models.CreateCaseAttributes, newCaseId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_CASES).
			Columns(
				"id",
				"inbox_id",
				"name",
				"org_id",
			).
			Values(
				newCaseId,
				createCaseAttributes.InboxId,
				createCaseAttributes.Name,
				createCaseAttributes.OrganizationId,
			),
	)
	return err
}

func (repo *MarbleDbRepository) UpdateCase(tx Transaction, updateCaseAttributes models.UpdateCaseAttributes) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Update(dbmodels.TABLE_CASES).Where(squirrel.Eq{"id": updateCaseAttributes.Id})

	if updateCaseAttributes.InboxId != "" {
		query = query.Set("inbox_id", updateCaseAttributes.InboxId)
	}

	if updateCaseAttributes.Name != "" {
		query = query.Set("name", updateCaseAttributes.Name)
	}

	if updateCaseAttributes.Status != "" {
		query = query.Set("status", updateCaseAttributes.Status)
	}

	_, err := pgTx.ExecBuilder(query)
	return err
}

func (repo *MarbleDbRepository) CreateCaseTag(tx Transaction, caseId, tagId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_CASE_TAGS).
			Columns(
				"case_id",
				"tag_id",
				"deleted_at",
			).
			Values(
				caseId,
				tagId,
				nil,
			),
	)
	return err
}

func (repo *MarbleDbRepository) ListCaseTagsByCaseId(tx Transaction, caseId string) ([]models.CaseTag, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseTagColumn...).
			From(dbmodels.TABLE_CASE_TAGS).
			Where(squirrel.Eq{"case_id": caseId}).
			Where(squirrel.Expr("deleted_at IS NULL")),
		dbmodels.AdaptCaseTag,
	)
}

func (repo *MarbleDbRepository) ListCaseTagsByTagId(tx Transaction, tagId string) ([]models.CaseTag, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseTagColumn...).
			From(dbmodels.TABLE_CASE_TAGS).
			Where(squirrel.Eq{"tag_id": tagId}).
			Where(squirrel.Expr("deleted_at IS NULL")),
		dbmodels.AdaptCaseTag,
	)
}

func (repo *MarbleDbRepository) SoftDeleteCaseTag(tx Transaction, tagId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := NewQueryBuilder().Update(dbmodels.TABLE_CASE_TAGS).Where(squirrel.Eq{"id": tagId})
	query = query.Set("deleted_at", squirrel.Expr("NOW()"))

	_, err := pgTx.ExecBuilder(query)
	return err
}

func casesWithRankColumns() []string {
	var columnAlias []string
	columnAlias = append(columnAlias, dbmodels.SelectCaseColumn...)

	columns := columnsNames("s", columnAlias)
	columns = append(columns, "rank_number", "total")
	return columns
}

func casesCoreQueryWithRank(pagination models.PaginationAndSorting) squirrel.SelectBuilder {
	orderCondition := fmt.Sprintf("c.%s %s, c.id %s", pagination.Sorting, pagination.Order, pagination.Order)

	query := squirrel.StatementBuilder.
		Select(dbmodels.SelectCaseColumn...).
		Column(fmt.Sprintf("RANK() OVER (ORDER BY %s) as rank_number", orderCondition)).
		Column("COUNT(*) OVER() AS total").
		From(dbmodels.TABLE_CASES + " AS c")

	// When fetching the previous page, we want the "last xx cases", so we need to reverse the order of the query,
	// select the xx items, then reverse again to put them back in the right order
	if pagination.OffsetId != "" && pagination.Previous {
		query = query.OrderBy(fmt.Sprintf("c.%s %s, c.id %s", pagination.Sorting, models.ReverseOrder(pagination.Order), models.ReverseOrder(pagination.Order)))
	} else {
		query = query.OrderBy(orderCondition)
	}
	return query
}

func applyCaseFilters(query squirrel.SelectBuilder, filters models.CaseFilters) squirrel.SelectBuilder {
	query = query.Where(squirrel.Eq{"c.org_id": filters.OrganizationId})

	if len(filters.Statuses) > 0 {
		query = query.Where(squirrel.Eq{"c.status": filters.Statuses})
	}
	if !filters.StartDate.IsZero() {
		query = query.Where(squirrel.GtOrEq{"c.created_at": filters.StartDate})
	}
	if !filters.EndDate.IsZero() {
		query = query.Where(squirrel.LtOrEq{"c.created_at": filters.EndDate})
	}
	if len(filters.InboxIds) > 0 {
		query = query.Where(squirrel.Eq{"c.inbox_id": filters.InboxIds})
	}
	return query
}

func applyCasesPagination(query squirrel.SelectBuilder, p models.PaginationAndSorting) (squirrel.SelectBuilder, error) {
	if p.OffsetId == "" {
		return query, nil
	}

	offsetSubquery, args, err := squirrel.
		Select("id", "org_id", string(p.Sorting)).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{"id": p.OffsetId}).
		ToSql()
	if err != nil {
		return query, err
	}
	query = query.Join(fmt.Sprintf("(%s) AS cursorRecord ON cursorRecord.org_id = s.org_id", offsetSubquery), args...)

	queryConditionBefore := fmt.Sprintf("s.%s < cursorRecord.%s OR (s.%s = cursorRecord.%s AND s.id < cursorRecord.id)", p.Sorting, p.Sorting, p.Sorting, p.Sorting)
	queryConditionAfter := fmt.Sprintf("s.%s > cursorRecord.%s OR (s.%s = cursorRecord.%s AND s.id > cursorRecord.id)", p.Sorting, p.Sorting, p.Sorting, p.Sorting)
	if p.Next {
		if p.Order == "DESC" {
			query = query.Where(queryConditionBefore)
		} else {
			query = query.Where(queryConditionAfter)
		}
	}

	if p.Previous {
		if p.Order == "DESC" {
			query = query.Where(queryConditionAfter)
		} else {
			query = query.Where(queryConditionBefore)
		}
	}

	return query, nil
}

/*
The most complex end query is like (case DESC, previous with offset)

SELECT

	c.id, c.created_at, c.inbox_id, c.name, c.org_id, c.status,
	array_agg(row(cc.id,cc.case_id,cc.user_id,cc.created_at) ORDER BY cc.created_at) FILTER (WHERE cc.id IS NOT NULL) as contributors,
	array_agg(row(ct.id,ct.case_id,ct.tag_id,ct.created_at,ct.deleted_at) ORDER BY ct.created_at) FILTER (WHERE ct.id IS NOT NULL) as tags,
	count(distinct d.id) as decisions_count,
	rank_number,
	total

FROM (

	SELECT
		s.id, s.created_at, s.inbox_id, s.name, s.org_id, s.status, rank_number, total
	FROM (
		SELECT
			id, created_at, inbox_id, name, org_id, status,
			RANK() OVER (ORDER BY c.created_at DESC, c.id DESC) as rank_number,
			COUNT(*) OVER() AS total
		FROM cases AS c
		WHERE c.org_id = $1
			AND c.inbox_id IN ($2,$3,$4,$5,$6,$7)
		ORDER BY c.created_at ASC, c.id ASC
	) AS s
	JOIN (
		SELECT
			id, org_id, created_at
		FROM cases
		WHERE id = $8
	) AS cursorRecord ON cursorRecord.org_id = s.org_id
	WHERE s.created_at > cursorRecord.created_at
		OR (s.created_at = cursorRecord.created_at AND s.id > cursorRecord.id)
	LIMIT 25

) AS c
LEFT JOIN case_contributors AS cc ON cc.case_id = c.id
LEFT JOIN case_tags AS ct ON ct.case_id = c.id AND ct.deleted_at IS NULL
LEFT JOIN decisions AS d ON d.case_id = c.id
GROUP BY c.id, c.created_at, c.inbox_id, c.name, c.org_id, c.status, rank_number, total
ORDER BY c.created_at DESC, c.id DESC
*/
func selectCasesWithJoinedFields(query squirrel.SelectBuilder, p models.PaginationAndSorting, fromSubquery bool) squirrel.SelectBuilder {
	groupBy := columnsNames("c", dbmodels.SelectCaseColumn)
	if fromSubquery {
		groupBy = append(groupBy, "rank_number", "total")
	}

	q := squirrel.StatementBuilder.
		Select(columnsNames("c", dbmodels.SelectCaseColumn)...).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY cc.created_at) FILTER (WHERE cc.id IS NOT NULL) as contributors",
				strings.Join(columnsNames("cc", dbmodels.SelectCaseContributorColumn), ","),
			),
		).
		Column(
			fmt.Sprintf(
				"array_agg(row(%s) ORDER BY ct.created_at) FILTER (WHERE ct.id IS NOT NULL) as tags",
				strings.Join(columnsNames("ct", dbmodels.SelectCaseTagColumn), ","),
			),
		).
		Column(fmt.Sprintf("(SELECT count(distinct d.id) FROM %s AS d WHERE d.case_id = c.id) AS decisions_count", dbmodels.TABLE_DECISIONS))
	if fromSubquery {
		q = q.Column("rank_number").Column("total")
	}

	if fromSubquery {
		q = q.FromSelect(query, "c")
	} else {
		q = q.From(dbmodels.TABLE_CASES + " AS c")
	}

	return q.
		LeftJoin(dbmodels.TABLE_CASE_CONTRIBUTORS + " AS cc ON cc.case_id = c.id").
		LeftJoin(dbmodels.TABLE_CASE_TAGS + " AS ct ON ct.case_id = c.id AND ct.deleted_at IS NULL").
		GroupBy(groupBy...).
		OrderBy(fmt.Sprintf("c.%s %s, c.id %s", p.Sorting, p.Order, p.Order)).
		PlaceholderFormat(squirrel.Dollar)

}

func (repo *MarbleDbRepository) CreateDbCaseFile(tx Transaction, createCaseFileAttributes models.CreateDbCaseFileInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_CASE_FILES).
			Columns(
				"id",
				"bucket_name",
				"case_id",
				"file_name",
				"file_reference",
			).
			Values(
				createCaseFileAttributes.Id,
				createCaseFileAttributes.BucketName,
				createCaseFileAttributes.CaseId,
				createCaseFileAttributes.FileName,
				createCaseFileAttributes.FileReference,
			),
	)
	return err
}

func (repo *MarbleDbRepository) GetCaseFileById(tx Transaction, caseFileId string) (models.CaseFile, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseFileColumn...).
			From(dbmodels.TABLE_CASE_FILES).
			Where(squirrel.Eq{"id": caseFileId}),
		dbmodels.AdaptCaseFile,
	)
}

func (repo *MarbleDbRepository) GetCasesFileByCaseId(tx Transaction, caseId string) ([]models.CaseFile, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseFileColumn...).
			From(dbmodels.TABLE_CASE_FILES).
			Where(squirrel.Eq{"case_id": caseId}).
			OrderBy("created_at DESC"),
		dbmodels.AdaptCaseFile,
	)
}
