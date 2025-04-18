package repositories

import (
	"context"
	"regexp"
	"strings"
	"testing"

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
		[]models.FilterWithType{})
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
		filters)
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
		filters)
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
		filters)
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

var normalizeWhitespaceRe = regexp.MustCompile(`\s+`)

func stripQuery(q string) (s string) {
	return strings.TrimSpace(normalizeWhitespaceRe.ReplaceAllString(q, " "))
}
