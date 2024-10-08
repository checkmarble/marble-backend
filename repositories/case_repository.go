package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) ListOrganizationCases(
	ctx context.Context,
	exec Executor,
	filters models.CaseFilters,
	pagination models.PaginationAndSorting,
) ([]models.CaseWithRank, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	coreQuery := casesCoreQueryWithRank(pagination)
	filteredCoreQuery := applyCaseFilters(coreQuery, filters)

	subquery := squirrel.StatementBuilder.
		Select(casesWithRankColumns()...).
		FromSelect(filteredCoreQuery, "s").
		Limit(uint64(pagination.Limit))

	var offsetCase models.Case
	if pagination.OffsetId != "" {
		var err error
		offsetCase, err = repo.GetCaseById(ctx, exec, pagination.OffsetId)
		if errors.Is(err, pgx.ErrNoRows) {
			return []models.CaseWithRank{}, errors.Wrap(models.NotFoundError,
				"No row found matching the provided offset case Id")
		} else if err != nil {
			return []models.CaseWithRank{}, errors.Wrap(err, "Error fetching offset case")
		}
	}

	paginatedSubquery, err := applyCasesPagination(subquery, pagination, offsetCase)
	if err != nil {
		return []models.CaseWithRank{}, err
	}
	queryWithJoinedFields := selectCasesWithJoinedFields(paginatedSubquery, pagination, true)

	count, err := countCases(ctx, exec, filters)
	if err != nil {
		return []models.CaseWithRank{}, errors.Wrap(err, "Error counting cases")
	}

	return SqlToListOfRow(ctx, exec, queryWithJoinedFields, func(row pgx.CollectableRow) (models.CaseWithRank, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DBPaginatedCases](row)
		if err != nil {
			return models.CaseWithRank{}, err
		}

		return dbmodels.AdaptCaseWithRank(db.DBCaseWithContributorsAndTags, db.RankNumber, count)
	})
}

func (repo *MarbleDbRepository) GetCaseById(ctx context.Context, exec Executor, caseId string) (models.Case, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Case{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectCasesWithJoinedFields(squirrel.SelectBuilder(NewQueryBuilder()), models.PaginationAndSorting{
			Sorting: models.CasesSortingCreatedAt,
		}, false).
			Where(squirrel.Eq{"c.id": caseId}),
		dbmodels.AdaptCaseWithContributorsAndTags,
	)
}

func (repo *MarbleDbRepository) CreateCase(ctx context.Context, exec Executor,
	createCaseAttributes models.CreateCaseAttributes, newCaseId string,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
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

func (repo *MarbleDbRepository) UpdateCase(ctx context.Context, exec Executor, updateCaseAttributes models.UpdateCaseAttributes) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_CASES).Where(squirrel.Eq{
		"id": updateCaseAttributes.Id,
	})

	if updateCaseAttributes.InboxId != "" {
		query = query.Set("inbox_id", updateCaseAttributes.InboxId)
	}

	if updateCaseAttributes.Name != "" {
		query = query.Set("name", updateCaseAttributes.Name)
	}

	if updateCaseAttributes.Status != "" {
		query = query.Set("status", updateCaseAttributes.Status)
	}

	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *MarbleDbRepository) CreateCaseTag(ctx context.Context, exec Executor, caseId, tagId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
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

func (repo *MarbleDbRepository) ListCaseTagsByCaseId(ctx context.Context, exec Executor, caseId string) ([]models.CaseTag, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(ctx, exec,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseTagColumn...).
			From(dbmodels.TABLE_CASE_TAGS).
			Where(squirrel.Eq{"case_id": caseId}).
			Where(squirrel.Expr("deleted_at IS NULL")),
		dbmodels.AdaptCaseTag,
	)
}

func (repo *MarbleDbRepository) ListCaseTagsByTagId(ctx context.Context, exec Executor, tagId string) ([]models.CaseTag, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(ctx, exec,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseTagColumn...).
			From(dbmodels.TABLE_CASE_TAGS).
			Where(squirrel.Eq{"tag_id": tagId}).
			Where(squirrel.Expr("deleted_at IS NULL")),
		dbmodels.AdaptCaseTag,
	)
}

func (repo *MarbleDbRepository) SoftDeleteCaseTag(ctx context.Context, exec Executor, tagId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().Update(dbmodels.TABLE_CASE_TAGS).Where(squirrel.Eq{"id": tagId})
	query = query.Set("deleted_at", squirrel.Expr("NOW()"))

	err := ExecBuilder(ctx, exec, query)
	return err
}

func casesWithRankColumns() []string {
	var columnAlias []string
	columnAlias = append(columnAlias, dbmodels.SelectCaseColumn...)

	columns := columnsNames("s", columnAlias)
	columns = append(columns, "rank_number")
	return columns
}

func casesCoreQueryWithRank(pagination models.PaginationAndSorting) squirrel.SelectBuilder {
	orderCondition := fmt.Sprintf("c.%s %s, c.id %s", pagination.Sorting, pagination.Order, pagination.Order)

	query := squirrel.StatementBuilder.
		Select(dbmodels.SelectCaseColumn...).
		Column(fmt.Sprintf("RANK() OVER (ORDER BY %s) as rank_number", orderCondition)).
		From(dbmodels.TABLE_CASES + " AS c")

	// When fetching the previous page, we want the "last xx cases", so we need to reverse the order of the query,
	// select the xx items, then reverse again to put them back in the right order
	if pagination.OffsetId != "" && pagination.Previous {
		query = query.OrderBy(fmt.Sprintf("c.%s %s, c.id %s", pagination.Sorting,
			models.ReverseOrder(pagination.Order), models.ReverseOrder(pagination.Order)))
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

func applyCasesPagination(query squirrel.SelectBuilder, p models.PaginationAndSorting, offsetCase models.Case) (squirrel.SelectBuilder, error) {
	if p.OffsetId == "" {
		return query, nil
	}

	var offsetField any
	switch p.Sorting {
	case models.CasesSortingCreatedAt:
		offsetField = offsetCase.CreatedAt
	default:
		// only pagination by created_at is allowed for now
		return query, fmt.Errorf("invalid sorting field: %w", models.BadParameterError)
	}

	queryConditionBefore := fmt.Sprintf("s.%s < ? OR (s.%s = ? AND s.id < ?)", p.Sorting, p.Sorting)
	queryConditionAfter := fmt.Sprintf("s.%s > ? OR (s.%s = ? AND s.id > ?)", p.Sorting, p.Sorting)
	args := []any{offsetField, offsetField, p.OffsetId}
	if p.Next {
		if p.Order == "DESC" {
			query = query.Where(queryConditionBefore, args...)
		} else {
			query = query.Where(queryConditionAfter, args...)
		}
	}

	if p.Previous {
		if p.Order == "DESC" {
			query = query.Where(queryConditionAfter, args...)
		} else {
			query = query.Where(queryConditionBefore, args...)
		}
	}

	return query, nil
}

func countCases(ctx context.Context, exec Executor, filters models.CaseFilters) (int, error) {
	subquery := NewQueryBuilder().
		Select("*").
		From(fmt.Sprintf("%s AS c", dbmodels.TABLE_CASES)).
		Limit(models.COUNT_ROWS_LIMIT)
	subquery = applyCaseFilters(subquery, filters)
	query := NewQueryBuilder().
		Select("COUNT(*)").
		FromSelect(subquery, "s")

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var count int
	err = exec.QueryRow(ctx, sql, args...).Scan(&count)
	return count, err
}

/*
The most complex end query is like (case DESC, previous with offset)

SELECT

	c.id, c.created_at, c.inbox_id, c.name, c.org_id, c.status,
	(SELECT array_agg(row(cc.id,cc.case_id,cc.user_id,cc.created_at) ORDER BY cc.created_at) as contributors FROM case_contributors WHERE cc.case_id=c.id),
	(SELECT array_agg(row(ct.id,ct.case_id,ct.tag_id,ct.created_at,ct.deleted_at) ORDER BY ct.created_at) as tags FROM case_tags WHERE ct.case_id=c.id AND ct.deleted_at IS NULL),
	count(distinct d.id) as decisions_count,
	rank_number

FROM (

	SELECT
		s.id, s.created_at, s.inbox_id, s.name, s.org_id, s.status, rank_number
	FROM (
		SELECT
			id, created_at, inbox_id, name, org_id, status,
			RANK() OVER (ORDER BY c.created_at DESC, c.id DESC) as rank_number
		FROM cases AS c
		WHERE c.org_id = $1
			AND c.inbox_id IN ($2,$3,$4,$5,$6,$7)
		ORDER BY c.created_at ASC, c.id ASC
	) AS s
	WHERE s.created_at > $8
		OR (s.created_at = $9 AND s.id > $10)
	LIMIT 25

) AS c
LEFT JOIN decisions AS d ON d.case_id = c.id
GROUP BY c.id, c.created_at, c.inbox_id, c.name, c.org_id, c.status, rank_number
ORDER BY c.created_at DESC, c.id DESC
*/
func selectCasesWithJoinedFields(query squirrel.SelectBuilder, p models.PaginationAndSorting, fromSubquery bool) squirrel.SelectBuilder {
	q := squirrel.StatementBuilder.
		Select(columnsNames("c", dbmodels.SelectCaseColumn)...).
		Column(
			fmt.Sprintf(
				"(SELECT array_agg(row(%s) ORDER BY cc.created_at) AS contributors FROM %s AS cc WHERE cc.case_id=c.id)",
				strings.Join(columnsNames("cc", dbmodels.SelectCaseContributorColumn), ","),
				dbmodels.TABLE_CASE_CONTRIBUTORS,
			),
		).
		Column(
			fmt.Sprintf(
				"(SELECT array_agg(row(%s) ORDER BY ct.created_at) AS tags FROM %s AS ct WHERE ct.case_id=c.id AND ct.deleted_at IS NULL)",
				strings.Join(columnsNames("ct", dbmodels.SelectCaseTagColumn), ","),
				dbmodels.TABLE_CASE_TAGS,
			),
		).
		Column(fmt.Sprintf("(SELECT count(distinct d.id) FROM %s AS d WHERE d.case_id = c.id AND d.org_id=c.org_id) AS decisions_count", dbmodels.TABLE_DECISIONS))
	if fromSubquery {
		q = q.Column("rank_number")
	}

	if fromSubquery {
		q = q.FromSelect(query, "c")
	} else {
		q = q.From(dbmodels.TABLE_CASES + " AS c")
	}

	return q.
		OrderBy(fmt.Sprintf("c.%s %s, c.id %s", p.Sorting, p.Order, p.Order)).
		PlaceholderFormat(squirrel.Dollar)
}

func (repo *MarbleDbRepository) CreateDbCaseFile(ctx context.Context, exec Executor,
	createCaseFileAttributes models.CreateDbCaseFileInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
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

func (repo *MarbleDbRepository) GetCaseFileById(ctx context.Context, exec Executor, caseFileId string) (models.CaseFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.CaseFile{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseFileColumn...).
			From(dbmodels.TABLE_CASE_FILES).
			Where(squirrel.Eq{"id": caseFileId}),
		dbmodels.AdaptCaseFile,
	)
}

func (repo *MarbleDbRepository) GetCasesFileByCaseId(ctx context.Context, exec Executor, caseId string) ([]models.CaseFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectCaseFileColumn...).
			From(dbmodels.TABLE_CASE_FILES).
			Where(squirrel.Eq{"case_id": caseId}).
			OrderBy("created_at DESC"),
		dbmodels.AdaptCaseFile,
	)
}
