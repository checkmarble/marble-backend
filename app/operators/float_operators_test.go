package operators

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

type DataAccessorFloatImpl struct{}

func (d *DataAccessorFloatImpl) GetPayloadField(fieldName string) interface{} {
	var val float64
	if f, err := strconv.ParseFloat(fieldName, 64); err == nil {
		val = f
	} else {
		return nil
	}
	return &val
}

func (d *DataAccessorFloatImpl) GetDbField(triggerTableName string, path []string, fieldName string) (interface{}, error) {
	var val pgtype.Float8
	if f, err := strconv.ParseFloat(fieldName, 64); err == nil {
		val = pgtype.Float8{Float64: f, Valid: true}
	} else {
		val = pgtype.Float8{Float64: 0, Valid: false}
	}
	return val, nil
}
func (d *DataAccessorFloatImpl) ValidateDbFieldReadConsistency(path []string, fieldName string) error {
	return nil
}

func TestLogicEvalFloat(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorFloat
		expected float64
	}
	dataAccessor := DataAccessorFloatImpl{}

	cases := []testCase{
		{
			name: "scalar",
			operator: &FloatValue{
				Value: 1,
			},
			expected: 1,
		},
		{
			name: "db field",
			operator: &DbFieldFloat{
				TriggerTableName: "table",
				Path:             []string{"1", "2"},
				FieldName:        "10.5",
			},
			expected: 10.5,
		},
		{
			name:     "payload field",
			operator: &PayloadFieldFloat{FieldName: "10.5"},
			expected: 10.5,
		},
		{
			name: "sum",
			operator: &SumFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}},
			},
			expected: 3.5,
		},
		{
			name: "product",
			operator: &ProductFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}},
			},
			expected: 2.5,
		},
		{
			name: "subtract",
			operator: &SubtractFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 2.5},
			},
			expected: -1.5,
		},
		{
			name: "divide",
			operator: &DivideFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 2.5},
			},
			expected: 0.4,
		},
		{
			name: "round",
			operator: &RoundFloat{
				operand: &FloatValue{Value: 2.5123},
				level:   2,
			},
			expected: 2.51,
		},
		{
			name: "round (negative)",
			operator: &RoundFloat{
				operand: &FloatValue{Value: 12345},
				level:   -2,
			},
			expected: 12300,
		},
	}
	asserts := assert.New(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.operator.Eval(&dataAccessor)

			if err != nil {
				t.Errorf("error: %v", err)
			}

			asserts.Equal(c.expected, got)
		})
	}
}

func TestLogicEvalErrorCaseFloat(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorFloat
	}
	dataAccessor := DataAccessorFloatImpl{}

	cases := []testCase{
		{
			name:     "Payload nil",
			operator: &PayloadFieldFloat{FieldName: "nil"},
		},
		{
			name: "Sum with nil",
			operator: &SumFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}, nil},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := c.operator.Eval(&dataAccessor)

			if err == nil {
				t.Errorf("Was expecting an error reading a null field")
			}

		})
	}
}

func TestMarshalUnMarshalFloat(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorFloat
	}
	dataAccessor := DataAccessorFloatImpl{}
	asserts := assert.New(t)

	cases := []testCase{
		{
			name:     "Scalar value",
			operator: &FloatValue{Value: 42.42},
		},
		{
			name: "Db field",
			operator: &DbFieldFloat{
				TriggerTableName: "table",
				Path:             []string{"1", "2"},
				FieldName:        "10.5",
			},
		},
		{
			name:     "Payload field",
			operator: &PayloadFieldFloat{FieldName: "10.5"},
		},
		{
			name: "Sum",
			operator: &SumFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}},
			},
		},
		{
			name: "Product",
			operator: &ProductFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}},
			},
		},
		{
			name: "Subtraction",
			operator: &SubtractFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 2.5},
			},
		},
		{
			name: "Division",
			operator: &DivideFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 2.5},
			},
		},
		{
			name: "Round",
			operator: &RoundFloat{
				operand: &FloatValue{Value: 2.5123},
				level:   2,
			},
		},
		{
			name: "Round (negative)",
			operator: &RoundFloat{
				operand: &FloatValue{Value: 7652.5123},
				level:   -2,
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

			rootOperator, err := UnmarshalOperatorFloat(JSONbytes)
			if err != nil {
				t.Errorf("error unmarshaling operator: %v", err)
			}
			fmt.Println(rootOperator)

			expected, err := c.operator.Eval(&dataAccessor)
			if err != nil {
				t.Errorf("error: %v", err)
			}
			got, err := rootOperator.Eval(&dataAccessor)
			if err != nil {
				t.Errorf("error: %v", err)
			}

			asserts.Equal(expected, got, "evaluated operator should be the same as the original")

		})
	}

}

func TestMarshallBoolOperatorsFloat(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorFloat
		expected string
	}
	asserts := assert.New(t)
	cases := []testCase{
		{
			name:     "scalar value",
			operator: &FloatValue{Value: 42.42},
			expected: `{"type":"FLOAT_SCALAR","staticData":{"value":42.42}}`,
		},
		{
			name: "sum",
			operator: &SumFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}},
			},
			expected: `{"type":"SUM_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":1}},{"type":"FLOAT_SCALAR","staticData":{"value":2.5}}]}`,
		},
		{
			name: "product",
			operator: &ProductFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}},
			},
			expected: `{"type":"PRODUCT_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":1}},{"type":"FLOAT_SCALAR","staticData":{"value":2.5}}]}`,
		},
		{
			name: "subtraction",
			operator: &SubtractFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 2.5},
			},
			expected: `{"type":"SUBTRACT_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":1}},{"type":"FLOAT_SCALAR","staticData":{"value":2.5}}]}`,
		},
		{
			name: "division",
			operator: &DivideFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 2.5},
			},
			expected: `{"type":"DIVIDE_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":1}},{"type":"FLOAT_SCALAR","staticData":{"value":2.5}}]}`,
		},
		{
			name: "Round",
			operator: &RoundFloat{
				operand: &FloatValue{Value: 2.5123},
				level:   2,
			},
			expected: `{"type":"ROUND_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":2.5123}}],"staticData":{"level":2}}`,
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

func TestUnmarshallBoolOperatorsFloat(t *testing.T) {
	type testCase struct {
		name     string
		expected OperatorFloat
		json     string
	}
	asserts := assert.New(t)
	cases := []testCase{
		{
			name:     "or with null",
			expected: &FloatValue{Value: 42.42},
			json:     `{"type":"FLOAT_SCALAR","staticData":{"value":42.42}}`,
		},
		{
			name: "sum",
			expected: &SumFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}},
			},
			json: `{"type":"SUM_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":1}},{"type":"FLOAT_SCALAR","staticData":{"value":2.5}}]}`,
		},
		{
			name: "product",
			expected: &ProductFloat{
				Operands: []OperatorFloat{&FloatValue{Value: 1}, &FloatValue{Value: 2.5}},
			},
			json: `{"type":"PRODUCT_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":1}},{"type":"FLOAT_SCALAR","staticData":{"value":2.5}}]}`,
		},
		{
			name: "subtraction",
			expected: &SubtractFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 2.5},
			},
			json: `{"type":"SUBTRACT_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":1}},{"type":"FLOAT_SCALAR","staticData":{"value":2.5}}]}`,
		},
		{
			name: "division",
			expected: &DivideFloat{
				Left:  &FloatValue{Value: 1},
				Right: &FloatValue{Value: 2.5},
			},
			json: `{"type":"DIVIDE_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":1}},{"type":"FLOAT_SCALAR","staticData":{"value":2.5}}]}`,
		},
		{
			name: "Round",
			expected: &RoundFloat{
				operand: &FloatValue{Value: 2.5123},
				level:   2,
			},
			json: `{"type":"ROUND_FLOAT","children":[{"type":"FLOAT_SCALAR","staticData":{"value":2.5123}}],"staticData":{"level":2}}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			operator, err := UnmarshalOperatorFloat([]byte(c.json))
			if err != nil {
				t.Errorf("error marshaling operator: %v", err)
			}
			asserts.Equal(c.expected, operator)
		})
	}
}
