package operators

import (
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

type DataAccessorStringImpl struct{}

func (d *DataAccessorStringImpl) GetPayloadField(fieldName string) (interface{}, error) {
	return &fieldName, nil
}

func (d *DataAccessorStringImpl) GetDbField(triggerTableName string, path []string, fieldName string) (interface{}, error) {
	val := pgtype.Text{String: fieldName, Valid: true}
	return val, nil
}

func TestLogicEvalString(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorString
		expected string
	}
	dataAccessor := DataAccessorStringImpl{}

	cases := []testCase{
		{
			name: "scalar",
			operator: &StringValue{
				Value: "abc",
			},
			expected: "abc",
		},

		{name: "db field",
			operator: &DbFieldString{
				TriggerTableName: "table",
				Path:             []string{"1", "2"},
				FieldName:        "test",
			},
			expected: "test",
		},
		{
			name:     "payload field",
			operator: &PayloadFieldString{FieldName: "test"},
			expected: "test",
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

func TestMarshalUnMarshalString(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorString
	}
	dataAccessor := DataAccessorStringImpl{}
	asserts := assert.New(t)

	cases := []testCase{
		{
			name:     "Scalar value",
			operator: &StringValue{Value: "abc"},
		},
		{
			name: "Db field",
			operator: &DbFieldString{
				TriggerTableName: "table",
				Path:             []string{"1", "2"},
				FieldName:        "test",
			},
		},
		{
			name:     "Payload field",
			operator: &PayloadFieldString{FieldName: "test"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			JSONbytes, err := c.operator.MarshalJSON()
			if err != nil {
				t.Errorf("error marshaling operator: %v", err)
			}

			t.Log("JSONbytes", string(JSONbytes))

			rootOperator, err := UnmarshalOperatorString(JSONbytes)
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

func TestMarshallBoolOperatorsString(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorString
		expected string
	}
	asserts := assert.New(t)
	cases := []testCase{
		{
			name:     "scalar value",
			operator: &StringValue{Value: "abc"},
			expected: `{"type":"STRING_CONSTANT","staticData":{"text":"abc"}}`,
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

func TestUnmarshallBoolOperatorsString(t *testing.T) {
	type testCase struct {
		name     string
		expected OperatorString
		json     string
	}
	asserts := assert.New(t)
	cases := []testCase{
		{
			name:     "string scalar value",
			expected: &StringValue{Value: "abc"},
			json:     `{"type":"STRING_CONSTANT","staticData":{"text":"abc"}}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			operator, err := UnmarshalOperatorString([]byte(c.json))
			if err != nil {
				t.Errorf("error marshaling operator: %v", err)
			}
			asserts.Equal(c.expected, operator)
		})
	}
}
