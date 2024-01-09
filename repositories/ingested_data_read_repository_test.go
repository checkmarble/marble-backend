package repositories

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

const expectedQueryDbFieldExpectedWithoutJoin string = "SELECT test_schema.second.int_var FROM test_schema.second " +
	"WHERE test_schema.second.object_id = $1 AND test_schema.second.valid_until = $2"

const expectedQueryDbFieldWithJoin string = "SELECT test_schema.third.int_var " +
	"FROM test_schema.second JOIN test_schema.third ON test_schema.second.id = test_schema.third.id " +
	"WHERE test_schema.second.object_id = $1 AND test_schema.second.valid_until = $2 AND test_schema.third.valid_until = $3"

const expectedQueryAggregatedWithoutFilter string = "SELECT AVG(int_var) FROM test_schema.first " +
	"WHERE test_schema.first.valid_until = $1"

const expectedQueryAggregatedWithFilter string = "SELECT AVG(int_var) FROM test_schema.first " +
	"WHERE test_schema.first.valid_until = $1 AND test_schema.first.int_var = $2 AND test_schema.first.bool_var <> $3"

type TransactionTest struct{}

func (tx TransactionTest) DatabaseSchema() models.DatabaseSchema {
	return models.DatabaseSchema{
		SchemaType: models.DATABASE_SCHEMA_TYPE_CLIENT,
		Schema:     "test_schema",
	}
}

type PayloadTest struct{}

func (pt PayloadTest) ReadFieldFromPayload(fieldName models.FieldName) (any, error) {
	return string(utils.DummyFieldNameId), nil
}

func (pt PayloadTest) ReadTableName() models.TableName {
	return utils.DummyTableNameFirst
}

func TestIngestedDataGetDbFieldWithoutJoin(t *testing.T) {
	path := []models.LinkName{models.LinkName(utils.DummyTableNameSecond)}

	query, err := createQueryDbForField(context.TODO(), TransactionTest{}, models.DbFieldReadParams{
		TriggerTableName: utils.DummyTableNameFirst,
		Path:             path,
		FieldName:        utils.DummyFieldNameForInt,
		DataModel:        utils.GetDummyDataModel(),
		Payload:          PayloadTest{},
	})
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 2) {
		assert.Equal(t, args[0], string(utils.DummyFieldNameId))
		assert.Equal(t, args[1], "Infinity")
	}
	assert.Equal(t, strings.ReplaceAll(sql, "\"", ""), expectedQueryDbFieldExpectedWithoutJoin)
}

func TestIngestedDataGetDbFieldWithJoin(t *testing.T) {
	path := []models.LinkName{models.LinkName(utils.DummyTableNameSecond), models.LinkName(utils.DummyTableNameThird)}

	query, err := createQueryDbForField(context.TODO(), TransactionTest{}, models.DbFieldReadParams{
		TriggerTableName: utils.DummyTableNameFirst,
		Path:             path,
		FieldName:        utils.DummyFieldNameForInt,
		DataModel:        utils.GetDummyDataModel(),
		Payload:          PayloadTest{},
	})
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 3) {
		assert.Equal(t, args[0], string(utils.DummyFieldNameId))
		assert.Equal(t, args[1], "Infinity")
		assert.Equal(t, args[2], "Infinity")
	}
	assert.Equal(t, strings.ReplaceAll(sql, "\"", ""), expectedQueryDbFieldWithJoin)
}

func TestIngestedDataQueryAggregatedValueWithoutFilter(t *testing.T) {
	query, err := createQueryAggregated(context.TODO(), TransactionTest{}, utils.DummyTableNameFirst, utils.DummyFieldNameForInt, ast.AGGREGATOR_AVG, []ast.Filter{})
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 1) {
		assert.Equal(t, args[0], "Infinity")
	}
	assert.Equal(t, strings.ReplaceAll(sql, "\"", ""), expectedQueryAggregatedWithoutFilter)
}

func TestIngestedDataQueryAggregatedValueWithFilter(t *testing.T) {
	filters := []ast.Filter{
		{
			TableName: string(utils.DummyTableNameFirst),
			FieldName: string(utils.DummyFieldNameForInt),
			Operator:  ast.FILTER_EQUAL,
			Value:     1,
		},
		{
			TableName: string(utils.DummyTableNameFirst),
			FieldName: string(utils.DummyFieldNameForBool),
			Operator:  ast.FILTER_NOT_EQUAL,
			Value:     true,
		},
	}

	query, err := createQueryAggregated(context.TODO(), TransactionTest{}, utils.DummyTableNameFirst, utils.DummyFieldNameForInt, ast.AGGREGATOR_AVG, filters)
	assert.Empty(t, err)
	sql, args, err := query.ToSql()
	assert.Empty(t, err)
	if assert.Len(t, args, 3) {
		assert.Equal(t, args[0], "Infinity")
		assert.Equal(t, args[1], 1)
		assert.Equal(t, args[2], true)
	}
	assert.Equal(t, strings.ReplaceAll(sql, "\"", ""), expectedQueryAggregatedWithFilter)
}
