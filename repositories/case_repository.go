package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	orderCond := orderConditionForCases(pagination)

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
	queryWithJoinedFields := selectCasesWithJoinedFields(paginatedSubquery, orderCond)

	return SqlToListOfRow(ctx, exec, queryWithJoinedFields, func(row pgx.CollectableRow) (models.CaseWithRank, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DBPaginatedCases](row)
		if err != nil {
			return models.CaseWithRank{}, err
		}
		return dbmodels.AdaptCaseWithRank(db.DBCaseWithContributorsAndTags, db.RankNumber)
	})
}

func (repo *MarbleDbRepository) GetCaseById(ctx context.Context, exec Executor, caseId string) (models.Case, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Case{}, err
	}

	query := NewQueryBuilder().
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
		Column(fmt.Sprintf("(SELECT count(distinct d.id) FROM %s AS d WHERE d.case_id = c.id AND d.org_id=c.org_id) AS decisions_count", dbmodels.TABLE_DECISIONS)).
		From(dbmodels.TABLE_CASES + " AS c").
		Where(squirrel.Eq{"c.id": caseId})

	return SqlToModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptCaseWithContributorsAndTags,
	)
}

func (repo *MarbleDbRepository) GetCaseMetadataById(ctx context.Context, exec Executor, caseId string) (models.CaseMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.CaseMetadata{}, err
	}
	query := NewQueryBuilder().
		Select(dbmodels.SelectCaseColumn...).
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{"id": caseId})
	c, err := SqlToModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptCase,
	)
	return c.GetMetadata(), err
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
	if updateCaseAttributes.Outcome != "" {
		query = query.Set("outcome", updateCaseAttributes.Outcome)
	}

	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *MarbleDbRepository) SnoozeCase(ctx context.Context, exec Executor, snoozeRequest models.CaseSnoozeRequest) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_CASES).
		Set("snoozed_until", snoozeRequest.Until).
		Where(squirrel.Eq{"id": snoozeRequest.CaseId})

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) UnsnoozeCase(ctx context.Context, exec Executor, caseId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_CASES).
		Set("snoozed_until", nil).
		Where(squirrel.Eq{"id": caseId})

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) AssignCase(ctx context.Context, exec Executor, id string, userId *models.UserId) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CASES).
		Set("assigned_to", userId).
		Where(squirrel.Eq{"id": id})

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) UnassignCase(ctx context.Context, exec Executor, id string) error {
	return repo.AssignCase(ctx, exec, id, nil)
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

	return squirrel.StatementBuilder.
		Select(dbmodels.SelectCaseColumn...).
		Column(fmt.Sprintf("RANK() OVER (ORDER BY %s) as rank_number", orderCondition)).
		From(dbmodels.TABLE_CASES + " AS c").
		OrderBy(orderCondition)
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
	if filters.Name != "" {
		query = query.Where("c.name % ?", filters.Name)
	}
	if !filters.IncludeSnoozed {
		query = query.Where(squirrel.Or{
			squirrel.Eq{"snoozed_until": nil},
			squirrel.LtOrEq{"snoozed_until": time.Now()},
		})
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

	if p.Order == models.SortingOrderDesc {
		query = query.Where(queryConditionBefore, args...)
	} else {
		query = query.Where(queryConditionAfter, args...)
	}

	return query, nil
}

func selectCasesWithJoinedFields(query squirrel.SelectBuilder, orderCond string) squirrel.SelectBuilder {
	return squirrel.StatementBuilder.
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
		Column(fmt.Sprintf("(SELECT count(distinct d.id) FROM %s AS d WHERE d.case_id = c.id AND d.org_id=c.org_id) AS decisions_count", dbmodels.TABLE_DECISIONS)).
		Column("rank_number").
		FromSelect(query, "c").
		OrderBy(orderCond).
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

func (repo *MarbleDbRepository) GetCasesWithPivotValue(ctx context.Context, exec Executor, orgId, pivotId, pivotValue string) ([]models.Case, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(columnsNames("c", dbmodels.SelectCaseColumn)...).
		Distinct().
		From(dbmodels.TABLE_DECISIONS + " d").
		InnerJoin("cases c on c.id = d.case_id").
		Where(squirrel.Eq{
			"d.org_id":      orgId,
			"d.pivot_value": pivotValue,
		}).
		OrderBy("c.created_at DESC").
		Limit(100)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptCase)
}

func orderConditionForCases(p models.PaginationAndSorting) string {
	return fmt.Sprintf("c.%s %s, c.id %s", p.Sorting, p.Order, p.Order)
}
