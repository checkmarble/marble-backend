package operators

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

type DataAccessorBoolImpl struct{}

func (d *DataAccessorBoolImpl) GetPayloadField(fieldName string) (interface{}, error) {
	if fieldName == "true" {
		return true, nil
	} else if fieldName == "false" {
		return false, nil
	} else {
		return nil, nil
	}
}

func (d *DataAccessorBoolImpl) GetDbField(ctx context.Context, triggerTableName string, path []string, fieldName string) (interface{}, error) {
	if fieldName == "true" {
		return true, nil
	} else if fieldName == "false" {
		return false, nil
	} else {
		return nil, nil
	}
}

func (d *DataAccessorBoolImpl) GetDbHandle() (db *pgxpool.Pool, schema string, err error) {
	return nil, "", nil
}

func TestLogicEval(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorBool
		expected bool
	}
	dataAccessor := DataAccessorBoolImpl{}

	cases := []testCase{
		{
			name: "1",
			operator: &EqBool{
				Left: &BoolValue{Value: true},
				Right: &EqBool{
					Left:  &BoolValue{Value: false},
					Right: &BoolValue{Value: false},
				},
			},
			expected: true,
		},
		{
			name: "2",
			operator: &EqBool{
				Left: &BoolValue{Value: true},
				Right: &EqBool{
					Left:  &BoolValue{Value: false},
					Right: &BoolValue{Value: true},
				},
			},
			expected: false},
		{
			name: "3",
			operator: &EqBool{
				Left: &BoolValue{Value: true},
				Right: &EqBool{
					Left:  &DbFieldBool{TriggerTableName: "a", Path: []string{"b", "c"}, FieldName: "true"},
					Right: &BoolValue{Value: true},
				},
			},
			expected: true,
		},
		{
			name: "4",
			operator: &EqBool{
				Left:  &BoolValue{Value: true},
				Right: &BoolValue{Value: false},
			},
			expected: false,
		},
		{
			name:     "Payload true",
			operator: &PayloadFieldBool{FieldName: "true"},
			expected: true,
		},
		{
			name:     "Payload false",
			operator: &PayloadFieldBool{FieldName: "false"},
			expected: false,
		},
		{
			name: "variadic and: 3 ops, true",
			operator: &And{
				Operands: []OperatorBool{&BoolValue{Value: true}, &BoolValue{Value: true}, &BoolValue{Value: true}},
			},
			expected: true,
		},
		{
			name: "variadic and: 3 ops, false",
			operator: &And{
				Operands: []OperatorBool{&BoolValue{Value: true}, &BoolValue{Value: true}, &BoolValue{Value: false}},
			},
			expected: false,
		},
		{
			name: "variadic and: 1 ops, false",
			operator: &And{
				Operands: []OperatorBool{&BoolValue{Value: false}},
			},
			expected: false,
		},
		{
			name: "variadic or: 3 ops, true",
			operator: &Or{
				Operands: []OperatorBool{&BoolValue{Value: false}, &BoolValue{Value: true}, &BoolValue{Value: false}},
			},
			expected: true,
		},
		{
			name: "variadic and: 3 ops, false",
			operator: &Or{
				Operands: []OperatorBool{&BoolValue{Value: false}, &BoolValue{Value: false}, &BoolValue{Value: false}},
			},
			expected: false,
		},
		{
			name: "variadic and: 1 ops, false",
			operator: &And{
				Operands: []OperatorBool{&BoolValue{Value: false}},
			},
			expected: false,
		},
		{
			name: "NOT true",
			operator: &Not{
				Child: &BoolValue{Value: true},
			},
			expected: false,
		},
		{
			name: "string equality",
			operator: &EqString{
				Left:  &StringValue{Value: "a"},
				Right: &StringValue{Value: "a"},
			},
			expected: true,
		},
		{
			name: "string equality",
			operator: &EqString{
				Left:  &StringValue{Value: "a"},
				Right: &StringValue{Value: "b"},
			},
			expected: false,
		},
		{
			name: "string is in",
			operator: &StringIsInList{
				Str:  &StringValue{Value: "a"},
				List: &StringListValue{Value: []string{"a", "b", "c"}},
			},
			expected: true,
		},
		{
			name: "string is in: not found",
			operator: &StringIsInList{
				Str:  &StringValue{Value: "z"},
				List: &StringListValue{Value: []string{"a", "b", "c"}},
			},
			expected: false,
		},
		{
			name: "string is in: not found (empty)",
			operator: &StringIsInList{
				Str:  &StringValue{Value: "z"},
				List: &StringListValue{Value: []string{}},
			},
			expected: false,
		},
		{
			name: "Greater than (float, true)",
			operator: &GreaterFloat{
				Left:  &FloatValue{Value: 10},
				Right: &FloatValue{Value: 5},
			},
			expected: true,
		},
		{
			name: "Greater than or equal (float, false)",
			operator: &GreaterOrEqualFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 5},
			},
			expected: false,
		},
		{
			name: "Greater than or equal (float, True)",
			operator: &GreaterOrEqualFloat{
				Left:  &FloatValue{Value: 5},
				Right: &FloatValue{Value: 5},
			},
			expected: true,
		},
		{
			name: "Equal (float, true)",
			operator: &EqualFloat{
				Left:  &FloatValue{Value: 5},
				Right: &FloatValue{Value: 5},
			},
			expected: true,
		},
		{
			name: "Constant float",
			operator: &BoolValue{
				Value: true,
			},
			expected: true,
		},
	}
	asserts := assert.New(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.operator.Eval(context.Background(), &dataAccessor)

			if err != nil {
				t.Errorf("error: %v on %s", err, c.name)
			}

			asserts.Equal(c.expected, got, c.name)
		})
	}
}

func TestLogicEvalErrorCase(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorBool
	}
	dataAccessor := DataAccessorBoolImpl{}

	cases := []testCase{
		{
			name:     "Payload nil",
			operator: &PayloadFieldBool{FieldName: "nil"},
		},
		{
			name:     "Payload nil",
			operator: &DbFieldBool{TriggerTableName: "transactions", Path: []string{"accounts"}, FieldName: "nil"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.operator.Eval(context.Background(), &dataAccessor)

			if err == nil {
				t.Errorf("Was expecting an error reading a null field")
			}

		})
	}
}

func TestMarshalUnMarshal(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorBool
	}
	dataAccessor := DataAccessorBoolImpl{}
	asserts := assert.New(t)

	cases := []testCase{
		{
			name: "Simple Equal",
			operator: &EqBool{
				Left:  &BoolValue{Value: false},
				Right: &BoolValue{Value: true},
			},
		},
		{
			name: "Larger tree",
			operator: &EqBool{
				Left: &BoolValue{Value: true},
				Right: &EqBool{
					Left:  &DbFieldBool{TriggerTableName: "transactinos", Path: []string{"accounts", "companies"}, FieldName: "true"},
					Right: &BoolValue{Value: true},
				},
			},
		},
		{
			name: "Variadic and",
			operator: &And{
				Operands: []OperatorBool{&BoolValue{Value: true}, &BoolValue{Value: true}, &BoolValue{Value: false}},
			},
		},
		{
			name: "Variadic or",
			operator: &Or{
				Operands: []OperatorBool{&BoolValue{Value: true}, &BoolValue{Value: true}, &BoolValue{Value: false}},
			},
		},
		{
			name: "Not true",
			operator: &Not{
				Child: &BoolValue{Value: true},
			},
		},
		{
			name: "String equality",
			operator: &EqString{
				Left:  &StringValue{Value: "a"},
				Right: &StringValue{Value: "abc"},
			},
		},
		{
			name: "String is in",
			operator: &StringIsInList{
				Str:  &StringValue{Value: "a"},
				List: &StringListValue{Value: []string{"a", "b", "c"}},
			},
		},
		{
			name: "String is in",
			operator: &StringIsInList{
				Str:  &StringValue{Value: "a"},
				List: &StringListValue{Value: []string{"a", "b", "c"}},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			JSONbytes, err := c.operator.MarshalJSON()
			if err != nil {
				t.Errorf("error marshaling operator: %v", err)
			}

			t.Log("JSONbytes", string(JSONbytes))

			rootOperator, err := UnmarshalOperatorBool(JSONbytes)
			if err != nil {
				t.Errorf("error unmarshaling operator: %v", err)
			}

			expected, err := c.operator.Eval(context.Background(), &dataAccessor)
			if err != nil {
				t.Errorf("error: %v", err)
			}
			got, err := rootOperator.Eval(context.Background(), &dataAccessor)
			if err != nil {
				t.Errorf("error: %v", err)
			}

			asserts.Equal(expected, got, "evaluated operator should be the same as the original")

		})
	}

}

func TestMarshallBoolOperators(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorBool
		expected string
	}
	asserts := assert.New(t)
	cases := []testCase{
		{
			name:     "true",
			operator: &BoolValue{Value: true},
			expected: `{"type":"BOOL_CONSTANT","staticData":{"value":true}}`,
		},
		{
			name:     "false",
			operator: &BoolValue{Value: false},
			expected: `{"type":"BOOL_CONSTANT","staticData":{"value":false}}`,
		},
		{
			name: "equal",
			operator: &EqBool{
				Left:  &BoolValue{Value: true},
				Right: &BoolValue{Value: false},
			},
			expected: `{"type":"EQUAL_BOOL","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}},{"type":"BOOL_CONSTANT","staticData":{"value":false}}]}`,
		},
		{
			name: "db field bool",
			operator: &DbFieldBool{
				TriggerTableName: "transactions",
				Path:             []string{"accounts", "companies"},
				FieldName:        "name",
			},
			expected: `{"type":"DB_FIELD_BOOL","staticData":{"triggerTableName":"transactions","path":["accounts","companies"],"fieldName":"name"}}`,
		},
		{
			name: "variadic and",
			operator: &And{
				Operands: []OperatorBool{&BoolValue{Value: true}, &BoolValue{Value: true}, &BoolValue{Value: false}},
			},
			expected: `{"type":"AND","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}},{"type":"BOOL_CONSTANT","staticData":{"value":true}},{"type":"BOOL_CONSTANT","staticData":{"value":false}}]}`,
		},
		{
			name: "variadic or",
			operator: &Or{
				Operands: []OperatorBool{&BoolValue{Value: true}, &BoolValue{Value: true}, &BoolValue{Value: false}},
			},
			expected: `{"type":"OR","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}},{"type":"BOOL_CONSTANT","staticData":{"value":true}},{"type":"BOOL_CONSTANT","staticData":{"value":false}}]}`,
		},
		{
			name: "not true",
			operator: &Not{
				Child: &BoolValue{Value: true},
			},
			expected: `{"type":"NOT","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}}]}`,
		},
		{
			name: "eq with null",
			operator: &EqBool{
				Left:  &BoolValue{Value: true},
				Right: nil,
			},
			expected: `{"type":"EQUAL_BOOL","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}},null]}`,
		},
		{
			name: "or with null",
			operator: &Or{
				Operands: []OperatorBool{&BoolValue{Value: true}, nil, &BoolValue{Value: false}},
			},
			expected: `{"type":"OR","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}},null,{"type":"BOOL_CONSTANT","staticData":{"value":false}}]}`,
		},
		{
			name: "String is in",
			operator: &StringIsInList{
				Str:  &StringValue{Value: "a"},
				List: &StringListValue{Value: []string{"a", "b", "c"}},
			},
			expected: `{"type":"STRING_IS_IN_LIST","children":[{"type":"STRING_CONSTANT","staticData":{"value":"a"}},{"type":"STRING_LIST_CONSTANT","staticData":{"value":["a","b","c"]}}]}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			JSONbytes, err := c.operator.MarshalJSON()
			if err != nil {
				t.Errorf("error marshaling operator: %v", err)
			}
			asserts.Equal(c.expected, string(JSONbytes))
		})
	}
}

func TestUnmarshallBoolOperators(t *testing.T) {
	type testCase struct {
		name     string
		expected OperatorBool
		json     string
	}
	asserts := assert.New(t)
	cases := []testCase{
		{
			name:     "true",
			expected: &BoolValue{Value: true},
			json:     `{"type":"BOOL_CONSTANT","staticData":{"value":true}}`,
		},
		{
			name:     "false",
			expected: &BoolValue{Value: false},
			json:     `{"type":"BOOL_CONSTANT","staticData":{"value":false}}`,
		},
		{
			name: "equal",
			expected: &EqBool{
				Left:  &BoolValue{Value: true},
				Right: &BoolValue{Value: false},
			},
			json: `{"type":"EQUAL_BOOL","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}},{"type":"BOOL_CONSTANT","staticData":{"value":false}}]}`,
		},
		{
			name: "equal",
			expected: &DbFieldBool{
				TriggerTableName: "transactions",
				Path:             []string{"accounts", "companies"},
				FieldName:        "name",
			},
			json: `{"type":"DB_FIELD_BOOL","staticData":{"triggerTableName":"transactions","path":["accounts","companies"],"fieldName":"name"}}`,
		},
		{
			name: "eq with null",
			expected: &EqBool{
				Left:  &BoolValue{Value: true},
				Right: nil,
			},
			json: `{"type":"EQUAL_BOOL","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}},null]}`,
		},
		{
			name: "or with null",
			expected: &Or{
				Operands: []OperatorBool{&BoolValue{Value: true}, nil, &BoolValue{Value: false}},
			},
			json: `{"type":"OR","children":[{"type":"BOOL_CONSTANT","staticData":{"value":true}},null,{"type":"BOOL_CONSTANT","staticData":{"value":false}}]}`,
		},
		{
			name: "String is in",
			expected: &StringIsInList{
				Str:  &StringValue{Value: "a"},
				List: &StringListValue{Value: []string{"a", "b", "c"}},
			},
			json: `{"type":"STRING_IS_IN_LIST","children":[{"type":"STRING_CONSTANT","staticData":{"value":"a"}},{"type":"STRING_LIST_CONSTANT","staticData":{"value":["a","b","c"]}}]}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			operator, err := UnmarshalOperatorBool([]byte(c.json))
			if err != nil {
				t.Errorf("error marshaling operator: %v", err)
			}
			asserts.Equal(c.expected, operator)
		})
	}
}

func TestInvalidOperators(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorBool
	}

	cases := []testCase{
		{
			name:     "empty and",
			operator: &And{},
		},
		{
			name:     "and with null",
			operator: &And{Operands: []OperatorBool{&BoolValue{Value: true}, nil}},
		},
		{
			name:     "and with null first",
			operator: &And{Operands: []OperatorBool{nil, &BoolValue{Value: true}, &BoolValue{Value: false}}},
		},
		{
			name: "eq",
			operator: &EqBool{
				Left:  &BoolValue{Value: true},
				Right: nil,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.operator.IsValid() {
				t.Errorf("operator should be invalid")
			}
		})
	}
}
