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
) ([]models.Case, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	orderCondition := fmt.Sprintf("c.boost is null %[1]s, c.%[2]s %[1]s, c.id %[1]s", pagination.Order, pagination.Sorting)

	// In the public API, we simply sort by created date, we do not boost cases.
	if filters.UseLinearOrdering {
		orderCondition = fmt.Sprintf("c.%[1]s %[2]s, c.id %[2]s", pagination.Sorting, pagination.Order)
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
		OrderBy(orderCondition).
		Limit(uint64(pagination.Limit))

		// Apply filters
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
		// Straight from the Postgres doc,
		// "Same as word_similarity, but forces extent boundaries to match word boundaries. Since we don't have cross-word trigrams,
		// this function actually returns greatest similarity between first string and any continuous extent of words of the second string."
		// Hence, the (presumably shorter) string received as input should be used as the first argument.
		query = query.Where("word_similarity(?, c.name) > ?", filters.Name, repo.similarityThreshold)
	}
	if !filters.IncludeSnoozed {
		query = query.Where(squirrel.Or{
			squirrel.Eq{"c.snoozed_until": nil},
			squirrel.LtOrEq{"c.snoozed_until": time.Now()},
		})
	}
	if filters.ExcludeAssigned {
		query = query.Where(squirrel.Eq{"c.assigned_to": nil})
	}
	if filters.AssigneeId != "" {
		query = query.Where(squirrel.Eq{"c.assigned_to": filters.AssigneeId})
	}
	if filters.TagId != nil {
		query = query.
			InnerJoin(dbmodels.TABLE_CASE_TAGS + " ct on ct.case_id = c.id AND ct.deleted_at IS NULL").
			Where(squirrel.Eq{"ct.tag_id": filters.TagId})
	}

	// Apply pagination, by fetching the offset case (error if not found)
	var offsetCase models.Case
	if pagination.OffsetId != "" {
		var err error
		offsetCase, err = repo.GetCaseById(ctx, exec, pagination.OffsetId)
		if errors.Is(err, pgx.ErrNoRows) {
			return []models.Case{}, errors.Wrap(models.NotFoundError,
				"No row found matching the provided offset case Id")
		} else if err != nil {
			return []models.Case{}, errors.Wrap(err, "Error fetching offset case")
		}
	}
	var err error
	query, err = applyCasesPagination(query, pagination, offsetCase, filters.UseLinearOrdering)
	if err != nil {
		return []models.Case{}, err
	}

	// Then, fetch the cases
	return SqlToListOfRow(ctx, exec, query, func(row pgx.CollectableRow) (models.Case, error) {
		db, err := pgx.RowToStructByPos[dbmodels.DBCaseWithContributorsAndTags](row)
		if err != nil {
			return models.Case{}, err
		}
		return dbmodels.AdaptCaseWithContributorsAndTags(db)
	})
}

func applyCasesPagination(query squirrel.SelectBuilder, p models.PaginationAndSorting,
	offsetCase models.Case, useLinearOrdering bool,
) (squirrel.SelectBuilder, error) {
	if p.OffsetId == "" {
		return query, nil
	}

	var offsetFieldVal any
	switch p.Sorting {
	case models.CasesSortingCreatedAt:
		offsetFieldVal = offsetCase.CreatedAt
	default:
		// only pagination by created_at is allowed for now
		return query, fmt.Errorf("invalid sorting field: %w", models.BadParameterError)
	}

	queryConditionBefore := fmt.Sprintf("(c.boost IS NULL, c.%s, c.id) < (?, ?, ?)", p.Sorting)
	queryConditionAfter := fmt.Sprintf("(c.boost IS NULL, c.%s, c.id) > (?, ?, ?)", p.Sorting)
	args := []any{offsetCase.Boost == nil, offsetFieldVal, p.OffsetId}

	if useLinearOrdering {
		queryConditionBefore = fmt.Sprintf("(c.%s, c.id) < (?, ?)", p.Sorting)
		queryConditionAfter = fmt.Sprintf("(c.%s, c.id) > (?, ?)", p.Sorting)
		args = args[1:]
	}

	if p.Order == models.SortingOrderDesc {
		query = query.Where(queryConditionBefore, args...)
	} else {
		query = query.Where(queryConditionAfter, args...)
	}

	return query, nil
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

func (repo *MarbleDbRepository) GetCaseReferents(ctx context.Context, exec Executor, caseIds []string) ([]models.CaseReferents, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("c.id as id").
		Column(fmt.Sprintf("case when c.assigned_to is null then null else row(%s) end as assignee",
			strings.Join(columnsNames("u", dbmodels.UserFields), ","))).
		Column(fmt.Sprintf("row(%s) as inbox", strings.Join(
			columnsNames("i", dbmodels.SelectInboxColumn), ","))).
		From(dbmodels.TABLE_CASES + " c").
		LeftJoin(dbmodels.TABLE_USERS + " u on u.id = c.assigned_to").
		InnerJoin(dbmodels.TABLE_INBOXES + " i on i.id = c.inbox_id").
		Where(squirrel.Eq{"c.id": caseIds})

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptCaseReferents)
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
				"assigned_to",
				"type",
			).
			Values(
				newCaseId,
				createCaseAttributes.InboxId,
				createCaseAttributes.Name,
				createCaseAttributes.OrganizationId,
				createCaseAttributes.AssigneeId,
				createCaseAttributes.Type.String(),
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

	if updateCaseAttributes.InboxId != nil {
		query = query.Set("inbox_id", *updateCaseAttributes.InboxId)
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
	if updateCaseAttributes.Boost != models.BoostUnboost {
		query = query.Set("boost", updateCaseAttributes.Boost)
	}
	if updateCaseAttributes.Boost == models.BoostUnboost {
		query = query.Set("boost", nil)
	}

	err := ExecBuilder(ctx, exec, query)
	return err
}

func (repo *MarbleDbRepository) BoostCase(ctx context.Context, exec Executor, id string, reason models.BoostReason) error {
	return repo.UpdateCase(ctx, exec, models.UpdateCaseAttributes{Id: id, Boost: reason})
}

func (repo *MarbleDbRepository) UnboostCase(ctx context.Context, exec Executor, id string) error {
	return repo.UpdateCase(ctx, exec, models.UpdateCaseAttributes{Id: id, Boost: models.BoostUnboost})
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
		Set("boost", nil).
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
			).Suffix("ON CONFLICT DO NOTHING"),
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

func (repo *MarbleDbRepository) CreateDbCaseFile(ctx context.Context, exec Executor,
	createCaseFileAttributes models.CreateDbCaseFileInput,
) (models.CaseFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.CaseFile{}, err
	}

	query := NewQueryBuilder().Insert(dbmodels.TABLE_CASE_FILES).
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
		).
		Suffix(fmt.Sprintf("returning %s", strings.Join(dbmodels.SelectCaseFileColumn, ", ")))

	return SqlToModel(ctx, exec, query, dbmodels.AdaptCaseFile)
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

func (repo *MarbleDbRepository) GetCasesWithPivotValue(ctx context.Context, exec Executor, orgId, pivotValue string) ([]models.Case, error) {
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

func (repo *MarbleDbRepository) GetContinuousScreeningCasesWithObjectAttr(ctx context.Context, exec Executor,
	orgId, objectType, objectId string,
) ([]models.Case, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(columnsNames("c", dbmodels.SelectCaseColumn)...).
		Distinct().
		From(dbmodels.TABLE_CONTINUOUS_SCREENINGS + " cs").
		InnerJoin(dbmodels.TABLE_CASES + " c on c.id = cs.case_id").
		Where(squirrel.Eq{
			"cs.org_id":      orgId,
			"cs.object_type": objectType,
			"cs.object_id":   objectId,
		}).
		OrderBy("c.created_at DESC").
		Limit(100)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptCase)
}

func (repo *MarbleDbRepository) EscalateCase(ctx context.Context, exec Executor, id, inboxId string) error {
	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_CASES).
		SetMap(map[string]any{
			"inbox_id":      inboxId,
			"snoozed_until": nil,
			"assigned_to":   nil,
			"boost":         models.BoostEscalated,
		}).
		Where(squirrel.Eq{"id": id})

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) GetNextCase(ctx context.Context, exec Executor, c models.Case) (string, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return "", err
	}

	query := NewQueryBuilder().
		Select("id").
		From(dbmodels.TABLE_CASES).
		Where(squirrel.And{
			squirrel.Eq{
				"org_id":      c.OrganizationId,
				"inbox_id":    c.InboxId,
				"assigned_to": nil,
			},
			squirrel.NotEq{"status": "closed"},
		}).
		OrderBy("boost is null", "created_at", "id").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return "", err
	}

	row := exec.QueryRow(ctx, sql, args...)

	var nextCaseId string

	if err := row.Scan(&nextCaseId); err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return "", errors.Wrap(models.NotFoundError, "no next case")
		default:
			return "", err
		}
	}

	return nextCaseId, nil
}

// Count the number of cases for each organization in the given time range, return a map of orgId to count
// From date is inclusive, to date is exclusive
func (repo *MarbleDbRepository) CountCasesByOrg(ctx context.Context, exec Executor,
	orgIds []string, from, to time.Time,
) (map[string]int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("org_id, count(*) as count").
		From(dbmodels.TABLE_CASES).
		Where(squirrel.Eq{"org_id": orgIds}).
		Where(squirrel.GtOrEq{"created_at": from}).
		Where(squirrel.Lt{"created_at": to}).
		GroupBy("org_id")

	return countByHelper(ctx, exec, query, orgIds)
}
