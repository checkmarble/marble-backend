package repositories

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

type TransactionTest struct{}

func (tx TransactionTest) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Schema:     "test_schema",
	}
}

func (tx TransactionTest) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return nil
}

func (tx TransactionTest) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (tx TransactionTest) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (tx TransactionTest) Begin(ctx context.Context) (Transaction, error) {
	return PgTx{}, nil
}

func (tx TransactionTest) Cache(ctx context.Context) *RedisExecutor {
	return nil
}

func TestIngestedDataGetDbFieldWithoutJoin(t *testing.T) {
	path := []string{utils.DummyTableNameSecond}

	nullFilter, query, err := createQueryDbForField(TransactionTest{}, models.DbFieldReadParams{
		TriggerTableName: utils.DummyTableNameFirst,
		Path:             path,
		FieldName:        utils.DummyFieldNameForInt,
		DataModel:        utils.GetDummyDataModel(),
		ClientObject: models.ClientObject{
			TableName: utils.DummyTableNameFirst,
			Data:      map[string]any{utils.DummyFieldNameId: utils.DummyFieldNameId},
		},
	})
	assert.False(t, nullFilter)
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 2) {
		assert.Equal(t, args[0], utils.DummyFieldNameId)
		assert.Equal(t, args[1], "Infinity")
	}
	expected := `
	SELECT table_1.int_var
	FROM "test_schema"."second" AS table_1
	WHERE table_1.id = $1
	AND table_1.valid_until = $2
	`
	assert.Equal(t, stripQuery(expected), stripQuery(sql))
}

func TestIngestedDataGetDbFieldWithJoin(t *testing.T) {
	path := []string{
		utils.DummyTableNameSecond,
		utils.DummyTableNameThird,
	}

	nullFilter, query, err := createQueryDbForField(TransactionTest{}, models.DbFieldReadParams{
		TriggerTableName: utils.DummyTableNameFirst,
		Path:             path,
		FieldName:        utils.DummyFieldNameForInt,
		DataModel:        utils.GetDummyDataModel(),
		ClientObject: models.ClientObject{
			TableName: utils.DummyTableNameFirst,
			Data:      map[string]any{utils.DummyFieldNameId: utils.DummyFieldNameId},
		},
	})
	assert.False(t, nullFilter)
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 3) {
		assert.Equal(t, args[0], utils.DummyFieldNameId)
		assert.Equal(t, args[1], "Infinity")
		assert.Equal(t, args[2], "Infinity")
	}
	expected := `
	SELECT table_2.int_var
	FROM "test_schema"."second" AS table_1
	JOIN "test_schema"."third" AS table_2 ON table_1.id = table_2.id
	WHERE table_1.id = $1
	AND table_1.valid_until = $2
	AND table_2.valid_until = $3
	`
	assert.Equal(t, stripQuery(expected), stripQuery(sql))
}

func TestIngestedDataQueryAggregatedValueWithoutFilter(t *testing.T) {
	query, err := createQueryAggregated(
		TransactionTest{},
		utils.DummyTableNameFirst,
		utils.DummyFieldNameForInt,
		models.Int,
		ast.AGGREGATOR_AVG,
		[]models.FilterWithType{},
		map[string]any{},
	)
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 1) {
		assert.Equal(t, args[0], "Infinity")
	}
	expected := `
	SELECT AVG(int_var)::float8
	FROM "test_schema"."first"
	WHERE "test_schema"."first".valid_until = $1
	`
	assert.Equal(t, stripQuery(expected), stripQuery(sql))
}

func TestIngestedDataQueryCountWithoutFilter(t *testing.T) {
	query, err := createQueryAggregated(
		TransactionTest{},
		utils.DummyTableNameFirst,
		utils.DummyFieldNameForInt,
		models.Int,
		ast.AGGREGATOR_COUNT,
		[]models.FilterWithType{},
		map[string]any{})
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 1) {
		assert.Equal(t, args[0], "Infinity")
	}
	expected := `
	SELECT COUNT(*)
	FROM "test_schema"."first"
	WHERE "test_schema"."first".valid_until = $1
	`
	assert.Equal(t, stripQuery(expected), stripQuery(sql))
}

func TestIngestedDataQueryAggregatedValueWithSimpleFilter(t *testing.T) {
	filters := []models.FilterWithType{
		{
			Filter: ast.Filter{
				TableName: utils.DummyTableNameFirst,
				FieldName: utils.DummyFieldNameForInt,
				Operator:  ast.FILTER_EQUAL,
				Value:     1,
			},
			FieldType: models.Int,
		},
		{
			Filter: ast.Filter{
				TableName: utils.DummyTableNameFirst,
				FieldName: utils.DummyFieldNameForBool,
				Operator:  ast.FILTER_NOT_EQUAL,
				Value:     true,
			},
			FieldType: models.Bool,
		},
	}

	query, err := createQueryAggregated(
		TransactionTest{},
		utils.DummyTableNameFirst,
		utils.DummyFieldNameForInt,
		models.Int,
		ast.AGGREGATOR_AVG,
		filters,
		map[string]any{})
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 3) {
		assert.Equal(t, args[0], "Infinity")
		assert.Equal(t, args[1], 1)
		assert.Equal(t, args[2], true)
	}
	expected := `
	SELECT AVG(int_var)::float8
	FROM "test_schema"."first"
	WHERE "test_schema"."first".valid_until = $1
	AND "test_schema"."first"."int_var" = $2
	AND "test_schema"."first"."bool_var" <> $3
	`
	assert.Equal(t, stripQuery(expected), stripQuery(sql))
}

func TestIngestedDataQueryAggregatedValueWithFuzzyMatchFilter_1(t *testing.T) {
	threshold := 0.5
	stringValue := "test"
	filters := []models.FilterWithType{
		{
			Filter: ast.Filter{
				TableName: "tableName",
				FieldName: "stringFieldName",
				Operator:  ast.FILTER_FUZZY_MATCH,
				Value: ast.FuzzyMatchOptions{
					Algorithm: "bag_of_words_similarity_db",
					Threshold: threshold,
					Value:     stringValue,
				},
			},
			FieldType: models.String,
		},
	}

	query, err := createQueryAggregated(
		TransactionTest{},
		"tableName",
		"stringFieldName",
		models.Int,
		ast.AGGREGATOR_COUNT,
		filters,
		map[string]any{})
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 5) {
		assert.Equal(t, args[0], "Infinity")
		assert.Equal(t, args[1], stringValue)
		assert.Equal(t, args[2], stringValue)
		assert.Equal(t, args[3], stringValue)
		assert.Equal(t, args[4], threshold)
	}

	expected := `
SELECT COUNT(*)
FROM "test_schema"."tableName"
WHERE "test_schema"."tableName".valid_until = $1
AND CASE
	WHEN length("test_schema"."tableName"."stringFieldName") < length($2) THEN word_similarity("test_schema"."tableName"."stringFieldName", $3)
	ELSE word_similarity($4, "test_schema"."tableName"."stringFieldName")
	END > $5
`

	assert.Equal(t, stripQuery(expected), stripQuery(sql))
}

func TestIngestedDataQueryAggregatedValueWithFuzzyMatchFilter_2(t *testing.T) {
	threshold := 0.5
	stringValue := "test"
	filters := []models.FilterWithType{
		{
			Filter: ast.Filter{
				TableName: "tableName",
				FieldName: "stringFieldName",
				Operator:  ast.FILTER_FUZZY_MATCH,
				Value: ast.FuzzyMatchOptions{
					Algorithm: "direct_string_similarity_db",
					Threshold: threshold,
					Value:     stringValue,
				},
			},
			FieldType: models.String,
		},
	}

	query, err := createQueryAggregated(
		TransactionTest{},
		"tableName",
		"stringFieldName",
		models.Int,
		ast.AGGREGATOR_COUNT,
		filters,
		map[string]any{})
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 9) {
		assert.Equal(t, args[0], "Infinity")
		assert.Equal(t, args[1], stringValue)
		assert.Equal(t, args[2], stringValue)
		assert.Equal(t, args[3], stringValue)
		assert.Equal(t, args[4], stringValue)
		assert.Equal(t, args[5], stringValue)
		assert.Equal(t, args[6], stringValue)
		assert.Equal(t, args[7], stringValue)
		assert.Equal(t, args[8], threshold)
	}

	expected := `
SELECT COUNT(*)
FROM "test_schema"."tableName"
WHERE "test_schema"."tableName".valid_until = $1
AND CASE
	WHEN GREATEST(LENGTH("test_schema"."tableName"."stringFieldName"), LENGTH($2)) < 6
		THEN 1.0 - (levenshtein("test_schema"."tableName"."stringFieldName", $3)::float / GREATEST(LENGTH("test_schema"."tableName"."stringFieldName"), LENGTH($4)))
	WHEN GREATEST(LENGTH("test_schema"."tableName"."stringFieldName"), LENGTH($5)) < 11
		THEN LEAST(1.0, SIMILARITY("test_schema"."tableName"."stringFieldName", $6)
			+ 0.05 * (11 - LEAST(1, LENGTH("test_schema"."tableName"."stringFieldName"), LENGTH($7))))
	ELSE SIMILARITY("test_schema"."tableName"."stringFieldName", $8)
	END > $9
`

	assert.Equal(t, stripQuery(expected), stripQuery(sql))
}

func TestIngestedDataQueryAggregatedValueWithIsMultipleOfFilter(t *testing.T) {
	tests := []struct {
		name         string
		value        any
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name:  "non-zero divisor",
			value: 10,
			expectedSQL: `
				SELECT COUNT(*)
				FROM "test_schema"."first"
				WHERE "test_schema"."first".valid_until = $1
				AND "test_schema"."first"."int_var"::numeric % $2 = 0
			`,
			expectedArgs: []any{"Infinity", 10},
		},
		{
			name:  "zero integer divisor",
			value: 0,
			expectedSQL: `
				SELECT COUNT(*)
				FROM "test_schema"."first"
				WHERE "test_schema"."first".valid_until = $1
				AND false
			`,
			expectedArgs: []any{"Infinity"},
		},
		{
			name:  "zero floating-point divisor",
			value: 0.0,
			expectedSQL: `
				SELECT COUNT(*)
				FROM "test_schema"."first"
				WHERE "test_schema"."first".valid_until = $1
				AND false
			`,
			expectedArgs: []any{"Infinity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := createQueryAggregated(
				TransactionTest{},
				utils.DummyTableNameFirst,
				utils.DummyFieldNameForInt,
				models.Int,
				ast.AGGREGATOR_COUNT,
				[]models.FilterWithType{{
					Filter: ast.Filter{
						TableName: utils.DummyTableNameFirst,
						FieldName: utils.DummyFieldNameForInt,
						Operator:  ast.FILTER_IS_MULTIPLE_OF,
						Value:     tt.value,
					},
					FieldType: models.Int,
				}},
				map[string]any{},
			)
			assert.NoError(t, err)

			sql, args, err := query.ToSql()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedArgs, args)
			assert.Equal(t, stripQuery(tt.expectedSQL), stripQuery(sql))
		})
	}
}

func TestIngestedDataQueryAggregatedValueWithTimestampExtractFilter(t *testing.T) {
	tests := []struct {
		name         string
		options      ast.TimestampExtractOptions
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name: "day of month",
			options: ast.TimestampExtractOptions{
				Part:     "day_of_month",
				Ranges:   [][2]int{{1, 10}},
				Timezone: "Europe/Paris",
			},
			expectedSQL: `
				SELECT COUNT(*)
				FROM "test_schema"."first"
				WHERE "test_schema"."first".valid_until = $1
				AND (extract(day from "test_schema"."first"."time_var" at time zone $2) between $3 and $4)
			`,
			expectedArgs: []any{"Infinity", "Europe/Paris", 1, 10},
		},
		{
			name: "day of week with multiple ranges",
			options: ast.TimestampExtractOptions{
				Part:     "day_of_week",
				Ranges:   [][2]int{{1, 2}, {6, 7}},
				Timezone: "Europe/Paris",
			},
			expectedSQL: `
				SELECT COUNT(*)
				FROM "test_schema"."first"
				WHERE "test_schema"."first".valid_until = $1
				AND (extract(isodow from "test_schema"."first"."time_var" at time zone $2) between $3 and $4
					OR extract(isodow from "test_schema"."first"."time_var" at time zone $5) between $6 and $7)
			`,
			expectedArgs: []any{"Infinity", "Europe/Paris", 1, 2, "Europe/Paris", 6, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := createQueryAggregated(
				TransactionTest{},
				utils.DummyTableNameFirst,
				utils.DummyFieldNameForInt,
				models.Int,
				ast.AGGREGATOR_COUNT,
				[]models.FilterWithType{{
					Filter: ast.Filter{
						TableName: utils.DummyTableNameFirst,
						FieldName: utils.DummyFieldNameForTimestamp,
						Operator:  ast.FILTER_TIMESTAMP_EXTRACT,
						Value:     tt.options,
					},
					FieldType: models.Timestamp,
				}},
				map[string]any{},
			)
			assert.NoError(t, err)

			sql, args, err := query.ToSql()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedArgs, args)
			assert.Equal(t, stripQuery(tt.expectedSQL), stripQuery(sql))
		})
	}
}

var normalizeWhitespaceRe = regexp.MustCompile(`\s+`)

func stripQuery(q string) (s string) {
	return strings.TrimSpace(normalizeWhitespaceRe.ReplaceAllString(q, " "))
}

func listIngestedObjectsTestTable() models.Table {
	return models.Table{
		Name: "transactions",
		Fields: map[string]models.Field{
			"object_id":  {Name: "object_id", DataType: models.String},
			"account_id": {Name: "account_id", DataType: models.String},
			"created_at": {Name: "created_at", DataType: models.Timestamp},
			"status":     {Name: "status", DataType: models.String},
			"label":      {Name: "label", DataType: models.String},
			"ip":         {Name: "ip", DataType: models.IpAddress},
			"amount":     {Name: "amount", DataType: models.Float},
			"count":      {Name: "count", DataType: models.Int},
		},
	}
}

func listIngestedObjectsTestParams(filters ...models.ExplorationFilter) models.ExplorationOptions {
	return models.ExplorationOptions{
		SourceTableName:   "accounts",
		FilterFieldName:   "account_id",
		FilterFieldValue:  models.NewStringOrNumberFromString("acc_1"),
		OrderingFieldName: "created_at",
		Filters:           filters,
	}
}

// Pins the query generated when no filters are passed (with a cursor): must not change with the addition of filters
func TestListIngestedObjectsQueryWithoutFilters(t *testing.T) {
	cursorTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	query, err := createQueryListIngestedObjects(TransactionTest{}, listIngestedObjectsTestTable(),
		listIngestedObjectsTestParams(), []string{"object_id", "valid_from"}, []any{cursorTime, "obj_1"}, 11)
	assert.NoError(t, err)
	sql, args, err := query.ToSql()
	assert.NoError(t, err)
	assert.Equal(t, []any{"acc_1", "Infinity", cursorTime, "obj_1"}, args)
	expected := `
	SELECT object_id, valid_from
	FROM "test_schema"."transactions"
	WHERE "test_schema"."transactions"."account_id" = $1
	AND "test_schema"."transactions".valid_until = $2
	AND ("test_schema"."transactions"."created_at", object_id) < ($3, $4)
	ORDER BY "test_schema"."transactions"."created_at" DESC, object_id DESC
	LIMIT 11
	`
	assert.Equal(t, stripQuery(expected), stripQuery(sql))
}

func TestListIngestedObjectsQueryWithFilters(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		filter   models.ExplorationFilter
		expected string
		args     []any
	}{
		{
			name: "in on strings",
			filter: models.ExplorationFilter{
				FieldName: "status",
				Operator:  models.ExplorationFilterIn, Values: []any{"VALIDATED", "PENDING"},
			},
			expected: `"test_schema"."transactions"."status" IN ($3,$4)`,
			args:     []any{"VALIDATED", "PENDING"},
		},
		{
			name: "in on ints",
			filter: models.ExplorationFilter{
				FieldName: "count",
				Operator:  models.ExplorationFilterIn, Values: []any{int64(1)},
			},
			expected: `"test_schema"."transactions"."count" IN ($3)`,
			args:     []any{int64(1)},
		},
		{
			name: "contains",
			filter: models.ExplorationFilter{
				FieldName: "label",
				Operator:  models.ExplorationFilterContains, Values: []any{"foo"},
			},
			expected: `"test_schema"."transactions"."label" ILIKE $3`,
			args:     []any{"%foo%"},
		},
		{
			name: "contains escapes LIKE wildcards and the escape character",
			filter: models.ExplorationFilter{
				FieldName: "label",
				Operator:  models.ExplorationFilterContains, Values: []any{`100%_o\k`},
			},
			expected: `"test_schema"."transactions"."label" ILIKE $3`,
			args:     []any{`%100\%\_o\\k%`},
		},
		{
			name: "contains on ip address",
			filter: models.ExplorationFilter{
				FieldName: "ip",
				Operator:  models.ExplorationFilterContains, Values: []any{"192.168."},
			},
			expected: `host("test_schema"."transactions"."ip") ILIKE $3`,
			args:     []any{"%192.168.%"},
		},
		{
			name: "greater or equal",
			filter: models.ExplorationFilter{
				FieldName: "amount",
				Operator:  models.ExplorationFilterGreaterOrEqual, Values: []any{100.0},
			},
			expected: `"test_schema"."transactions"."amount" >= $3`,
			args:     []any{100.0},
		},
		{
			name: "lesser or equal",
			filter: models.ExplorationFilter{
				FieldName: "amount",
				Operator:  models.ExplorationFilterLesserOrEqual, Values: []any{500.5},
			},
			expected: `"test_schema"."transactions"."amount" <= $3`,
			args:     []any{500.5},
		},
		{
			name: "lesser on timestamp",
			filter: models.ExplorationFilter{
				FieldName: "created_at",
				Operator:  models.ExplorationFilterLesser, Values: []any{to},
			},
			expected: `"test_schema"."transactions"."created_at" < $3`,
			args:     []any{to},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := createQueryListIngestedObjects(TransactionTest{}, listIngestedObjectsTestTable(),
				listIngestedObjectsTestParams(tt.filter), []string{"object_id"}, nil, 10)
			assert.NoError(t, err)
			sql, args, err := query.ToSql()
			assert.NoError(t, err)
			assert.Equal(t, append([]any{"acc_1", "Infinity"}, tt.args...), args)
			expected := `
			SELECT object_id
			FROM "test_schema"."transactions"
			WHERE "test_schema"."transactions"."account_id" = $1
			AND "test_schema"."transactions".valid_until = $2
			AND ` + tt.expected + `
			ORDER BY "test_schema"."transactions"."created_at" DESC, object_id DESC
			LIMIT 10
			`
			assert.Equal(t, stripQuery(expected), stripQuery(sql))
		})
	}

	t.Run("several filters with a cursor", func(t *testing.T) {
		query, err := createQueryListIngestedObjects(TransactionTest{}, listIngestedObjectsTestTable(),
			listIngestedObjectsTestParams(
				models.ExplorationFilter{
					FieldName: "created_at",
					Operator:  models.ExplorationFilterGreaterOrEqual, Values: []any{from},
				},
				models.ExplorationFilter{
					FieldName: "created_at",
					Operator:  models.ExplorationFilterLesser, Values: []any{to},
				},
			), []string{"object_id"}, []any{to, "obj_1"}, 10)
		assert.NoError(t, err)
		sql, args, err := query.ToSql()
		assert.NoError(t, err)
		assert.Equal(t, []any{"acc_1", "Infinity", to, "obj_1", from, to}, args)
		expected := `
		SELECT object_id
		FROM "test_schema"."transactions"
		WHERE "test_schema"."transactions"."account_id" = $1
		AND "test_schema"."transactions".valid_until = $2
		AND ("test_schema"."transactions"."created_at", object_id) < ($3, $4)
		AND "test_schema"."transactions"."created_at" >= $5
		AND "test_schema"."transactions"."created_at" < $6
		ORDER BY "test_schema"."transactions"."created_at" DESC, object_id DESC
		LIMIT 10
		`
		assert.Equal(t, stripQuery(expected), stripQuery(sql))
	})
}

func TestListIngestedObjectsQueryRejectsInvalidFilters(t *testing.T) {
	tests := []struct {
		name        string
		filter      models.ExplorationFilter
		errContains string
	}{
		{"unknown field", models.ExplorationFilter{
			FieldName: `x" = '' OR 1=1 --`,
			Operator:  models.ExplorationFilterIn, Values: []any{"a"},
		}, "unknown filter field"},
		{"no values", models.ExplorationFilter{
			FieldName: "status", Operator: models.ExplorationFilterIn,
		}, "no value for filter"},
		{"unknown operator", models.ExplorationFilter{
			FieldName: "status", Operator: "=", Values: []any{"a"},
		}, "unknown filter operator"},
		{"contains with non string value", models.ExplorationFilter{
			FieldName: "label",
			Operator:  models.ExplorationFilterContains, Values: []any{1.0},
		}, "invalid value for contains filter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := createQueryListIngestedObjects(TransactionTest{}, listIngestedObjectsTestTable(),
				listIngestedObjectsTestParams(tt.filter), []string{"object_id"}, nil, 10)
			assert.ErrorContains(t, err, tt.errContains)
		})
	}
}
