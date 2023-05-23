package operators

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type DataAccessorStringListImpl struct{}

func (d *DataAccessorStringListImpl) GetPayloadField(fieldName string) interface{} {
	return nil
}

func (d *DataAccessorStringListImpl) GetDbField(triggerTableName string, path []string, fieldName string) (interface{}, error) {
	return nil, nil
}

func TestLogicEvalStringList(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorStringList
		expected []string
	}
	dataAccessor := DataAccessorStringImpl{}

	cases := []testCase{
		{
			name: "scalar",
			operator: &StringListValue{
				Strings: []string{"abc", "def"},
			},
			expected: []string{"abc", "def"},
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

func TestMarshalUnMarshalStringList(t *testing.T) {
	type testCase struct {
		name     string
		operator OperatorStringList
	}
	dataAccessor := DataAccessorStringImpl{}
	asserts := assert.New(t)

	cases := []testCase{
		{
			name:     "Constant value",
			operator: &StringListValue{Strings: []string{"abc", "def"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			JSONbytes, err := c.operator.MarshalJSON()
			if err != nil {
				t.Errorf("error marshaling operator: %v", err)
			}

			t.Log("JSONbytes", string(JSONbytes))

			rootOperator, err := UnmarshalOperatorStringList(JSONbytes)
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
